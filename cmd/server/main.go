package main

import (
	"context"
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

func main() {
	if os.Geteuid() == 0 {
		log.Fatalf("Error: Go-Live Orchestrator should not be run as root.")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Fatalf("Error: ffmpeg is not installed or not found in PATH.")
	}

	log.Println("Starting Go-Live Orchestrator...")

	// 1. Setup Database
	// Using a default DSN for now, this should ideally come from env or config
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/live_db?sslmode=disable"
	}
	dbConn, err := db.InitDB(dsn)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v. Logging to DB will be disabled.", err)
	} else {
		defer dbConn.Close()
		if err := db.SetupTables(dbConn); err != nil {
			log.Fatalf("Failed to setup database tables: %v", err)
		}
	}

	// 2. Setup Context for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Setup Process Manager
	pm := engine.NewProcessManager(dbConn)
	defer pm.Stop()

	// 4. Setup Config Watcher
	configPath := "configs/config.yaml"
	if os.Getenv("CONFIG_PATH") != "" {
		configPath = os.Getenv("CONFIG_PATH")
	}

	// Load initial config
	initialConfig, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load initial configuration from %s: %v", configPath, err)
	}

	// Start Process Manager with initial config
	if err := pm.Start(ctx, initialConfig); err != nil {
		log.Fatalf("Failed to start process manager: %v", err)
	}

	// Define watcher callback
	onChange := func(newCfg *models.Config, diff config.DiffResult) {
		log.Printf("Configuration changed. Restart required: %v, Filter update required: %v", diff.RequiresRestart, diff.RequiresFilterUpdate)
		if diff.RequiresRestart {
			log.Println("Restarting FFmpeg process due to configuration change...")
			pm.Stop()
			if err := pm.Start(ctx, newCfg); err != nil {
				log.Printf("Failed to restart process manager: %v", err)
			}
		} else if diff.RequiresFilterUpdate {
			log.Println("Filter update required. Currently requiring full restart until live-update is implemented.")
			// Fallback to restart for now if we haven't implemented filter live-updates yet
			pm.Stop()
			if err := pm.Start(ctx, newCfg); err != nil {
				log.Printf("Failed to restart process manager: %v", err)
			}
		}
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

	// pm.Stop() and watcher.Stop() will be called via defer
	log.Println("Shutdown complete.")
}
