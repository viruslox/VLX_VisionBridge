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

// CheckFFmpeg checks if ffmpeg is available in the system PATH.
func CheckFFmpeg(lookPath func(string) (string, error)) error {
	if _, err := lookPath("ffmpeg"); err != nil {
		return fmt.Errorf("Error: ffmpeg is not installed or not found in PATH.")
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
		log.Println("Restarting FFmpeg process due to configuration change...")
		pm.UpdateConfig(newCfg)
	} else if diff.RequiresFilterUpdate {
		log.Println("Filter update required. Applying live-update via ZMQ...")
		pm.UpdateFilter(newCfg)
	}
}

// managePulseAudio starts the PulseAudio daemon and returns a cleanup closure.
func managePulseAudio() func() {
	// Kill any existing/stale pulseaudio daemon
	_ = exec.Command("pulseaudio", "-k").Run()

	// Start the daemon
	if err := exec.Command("pulseaudio", "-D", "--exit-idle-time=-1").Run(); err != nil {
		log.Printf("Warning: Failed to start PulseAudio daemon: %v", err)
	} else {
		log.Println("PulseAudio daemon started successfully.")
	}

	return func() {
		log.Println("Stopping PulseAudio daemon...")
		_ = exec.Command("pulseaudio", "-k").Run()
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

	if err := CheckFFmpeg(exec.LookPath); err != nil {
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
	cleanupPulse := managePulseAudio()
	defer cleanupPulse()
	pm := engine.NewProcessManager(dbConn)
	defer pm.Stop()

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

	// 5. Handle OS Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	cancel() // Cancel context

	log.Println("Shutdown complete.")
}
