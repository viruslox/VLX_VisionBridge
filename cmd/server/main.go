package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/user/VLX_VisionBridge/internal/config"
	"github.com/user/VLX_VisionBridge/internal/controlapi"
	"github.com/user/VLX_VisionBridge/internal/db"
	"github.com/user/VLX_VisionBridge/internal/engine"
	"github.com/user/VLX_VisionBridge/internal/models"
)

// ProcessUpdater defines the interface for updating configuration.
type ProcessUpdater interface {
	UpdateConfig(config *models.Config)
	UpdateFilter(config *models.Config)
}

// CheckEUID checks if the process is running as root.
func CheckEUID(euid int) error {
	if euid == 0 {
		return fmt.Errorf("Error: VLX VisionBridge should not be run as root.")
	}
	return nil
}

// CheckGStreamer checks if gst-launch-1.0 is available in the system PATH.
func CheckGStreamer(lookPath func(string) (string, error)) error {
	if _, err := lookPath("gst-launch-1.0"); err != nil {
		return fmt.Errorf("Error: gst-launch-1.0 is not installed or not found in PATH.")
	}
	return nil
}

// ResolveConfigPath determines the path to the configuration file.
func ResolveConfigPath(envPath string) string {
	if envPath != "" {
		return envPath
	}
	if _, err := os.Stat("configs/visionbridge.settings"); err == nil {
		return "configs/visionbridge.settings"
	}
	return "/opt/VLX_VisionBridge/etc/visionbridge.settings"
}

// ResolveDSN determines the database DSN to use.
func ResolveDSN(envDSN string, configDSN string) string {
	if envDSN != "" {
		return envDSN
	}
	return configDSN
}

// HandleConfigChange processes configuration changes.
func HandleConfigChange(pm ProcessUpdater, newCfg *models.Config, diff config.DiffResult) {
	log.Printf("Configuration changed. Restart required: %v, Filter update required: %v", diff.RequiresRestart, diff.RequiresFilterUpdate)
	if diff.RequiresRestart {
		log.Println("Restarting GStreamer process due to configuration change...")
		pm.UpdateConfig(newCfg)
	} else if diff.RequiresFilterUpdate {
		log.Println("Filter update required. Applying live-update via IPC...")
		pm.UpdateFilter(newCfg)
	}
}

// SetupDatabase initializes the database connection and ensures tables are set up.
func SetupDatabase(dsn string) *sql.DB {
	if dsn == "" {
		log.Println("Warning: No database DSN provided. Logging to DB will be disabled.")
		return nil
	}

	dbConn, err := db.InitDB(dsn)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v. Logging to DB will be disabled.", err)
		return nil
	}

	if err := db.SetupTemplatesTable(dbConn); err != nil {
		log.Printf("Warning: failed to set up templates table: %v", err)
	}

	if err := db.SetupTables(dbConn); err != nil {
		dbConn.Close()
		log.Fatalf("Failed to setup database tables: %v", err)
	}

	return dbConn
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		if os.Geteuid() != 0 {
			log.Fatalf("Error: 'install' must be run as root.")
		}
		Install()
		os.Exit(0)
	}

	if err := CheckEUID(os.Geteuid()); err != nil {
		log.Fatalf("%v", err)
	}

	if err := CheckGStreamer(exec.LookPath); err != nil {
		log.Fatalf("%v", err)
	}

	log.Println("Starting VLX VisionBridge...")

	// 1. Setup Configuration
	configPath := ResolveConfigPath(os.Getenv("CONFIG_PATH"))

	// Load initial config
	initialConfig, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load initial configuration from %s: %v", configPath, err)
	}

	// 2. Setup Database
	dsn := ResolveDSN(os.Getenv("DATABASE_URL"), initialConfig.Database.DSN)
	dbConn := SetupDatabase(dsn)
	if dbConn != nil {
		defer dbConn.Close()
	}

	// 3. Setup Context for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Setup Process Manager
	pm := engine.NewProcessManager(dbConn)
	defer pm.Stop()

	go engine.StartWebRTCServer(initialConfig)

	// Start Process Manager with initial config
	if err := pm.Start(ctx, initialConfig); err != nil {
		log.Fatalf("Failed to start process manager: %v", err)
	}

	// Define watcher callback
	onChange := func(newCfg *models.Config, diff config.DiffResult) {
		HandleConfigChange(pm, newCfg, diff)
	}

	watcher := config.NewWatcher(configPath, onChange)
	if err := watcher.Start(ctx); err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}
	defer watcher.Stop()

	// Start the always-on control/status API used by the web GUI. It runs
	// regardless of pipeline state so the GUI can always reach the backend, and
	// dispatches toggles through the engine's existing control handler.
	shutdownVB := func() {
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Signal(syscall.SIGTERM)
		}
	}
	var ctrlAPI *controlapi.Server
	if initialConfig.ControlAPI.Enable {
		ctrlAPI = controlapi.New(
			pm,
			dbConn,
			initialConfig.ControlAPI.BindAddr,
			initialConfig.ControlAPI.Port,
			initialConfig.ControlAPI.User,
			initialConfig.ControlAPI.Pass,
			initialConfig.ControlAPI.LogUnit,
			shutdownVB,
		)
		ctrlAPI.Start()
	}

	// 5. Handle OS Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	cancel() // Cancel context

	if ctrlAPI != nil {
		ctrlAPI.Stop()
	}

	log.Println("Shutdown complete.")
}
