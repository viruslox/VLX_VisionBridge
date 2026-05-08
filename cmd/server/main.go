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

	"github.com/user/go-live-orchestrator/internal/config"
	"github.com/user/go-live-orchestrator/internal/db"
	"github.com/user/go-live-orchestrator/internal/engine"
	"github.com/user/go-live-orchestrator/internal/models"
)

// ProcessUpdater defines the interface for updating configuration.
type ProcessUpdater interface {
	UpdateConfig(config *models.Config)
}

// CheckEUID checks if the process is running as root.
func CheckEUID(euid int) error {
	if euid == 0 {
		return fmt.Errorf("Error: Go-Live Orchestrator should not be run as root.")
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
	return "configs/config.yaml"
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
		log.Println("Filter update required. Currently requiring full restart until live-update is implemented.")
		pm.UpdateConfig(newCfg)
	}
}

func main() {
	if err := CheckEUID(os.Geteuid()); err != nil {
		log.Fatalf("%v", err)
	}

	if err := CheckFFmpeg(exec.LookPath); err != nil {
		log.Fatalf("%v", err)
	}

	log.Println("Starting Go-Live Orchestrator...")

	// 1. Setup Configuration
	configPath := ResolveConfigPath(os.Getenv("CONFIG_PATH"))

	// Load initial config
	initialConfig, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load initial configuration from %s: %v", configPath, err)
	}

	// 2. Setup Database
	var dbConn *sql.DB
	dsn := ResolveDSN(os.Getenv("DATABASE_URL"), initialConfig.Database.DSN)

	if dsn != "" {
		var err error
		dbConn, err = db.InitDB(dsn)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v. Logging to DB will be disabled.", err)
			dbConn = nil
		} else {
			defer dbConn.Close()
			if err := db.SetupTables(dbConn); err != nil {
				log.Fatalf("Failed to setup database tables: %v", err)
			}
		}
	} else {
		log.Println("Warning: No database DSN provided. Logging to DB will be disabled.")
	}

	// 3. Setup Context for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Setup Process Manager
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
