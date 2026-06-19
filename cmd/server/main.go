package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

// managePulseAudio starts an isolated, headless PulseAudio daemon and returns a cleanup closure.
func managePulseAudio(cfg *models.Config) func() {
	// Assicura la presenza della cartella var per le configurazioni temporanee
	_ = os.MkdirAll("/opt/VLX_VisionBridge/var", 0755)

	socketPath := "/tmp/vlx_visionbridge_pulse.socket"
	configPath := "/opt/VLX_VisionBridge/var/pulse_isolated.pa"

	// Pulisce eventuali socket residui da vecchi crash
	_ = os.Remove(socketPath)

	// Profilo PulseAudio headless: non dichiarando driver ALSA/oss non entrerà 
	// MAI in conflitto con PipeWire o altre istanze utente attive sull'host.
	pulseConfig := fmt.Sprintf(
		"load-module module-native-protocol-unix auth-anonymous=1 socket=%s\n"+
			"load-module module-null-sink sink_name=vlx_chromium_sink sink_properties=device.description=vlx_chromium_sink\n"+
			"set-default-sink vlx_chromium_sink\n",
		socketPath,
	)

	if err := os.WriteFile(configPath, []byte(pulseConfig), 0644); err != nil {
		log.Printf("Error: Failed to write isolated PulseAudio config: %v", err)
	}

	// Determina la risoluzione dinamica dello schermo virtuale
	screenRes := "1920x1080" // Fallback
	if cfg != nil && cfg.Input.Resolution != "" {
		screenRes = cfg.Input.Resolution
	}

	log.Printf("Starting Xvfb on display :99 with resolution %s...", screenRes)
	xvfbCmd := exec.Command("Xvfb", ":99", "-screen", "0", fmt.Sprintf("%sx24", screenRes))
	if err := xvfbCmd.Start(); err != nil {
		log.Printf("Warning: Failed to start Xvfb: %v", err)
	}
	os.Setenv("DISPLAY", ":99")

	log.Println("Starting completely isolated PulseAudio server instance for software loopback audio...")
	pulseCmd := exec.Command("pulseaudio", "-n", "-F", configPath, "--exit-idle-time=-1", "--daemonize=no")
	
	var pulseStderr strings.Builder
	pulseCmd.Stderr = &pulseStderr

	if err := pulseCmd.Start(); err != nil {
		log.Printf("Warning: Failed to launch standalone PulseAudio process: %v", err)
	}

	// Forziamo l'ambiente corrente e tutti i processi figli (pactl, chromium, ffmpeg)
	// ad usare esclusivamente il nostro socket UNIX privato, bypassando D-Bus e XDG_RUNTIME
	os.Setenv("PULSE_SERVER", fmt.Sprintf("unix:%s", socketPath))

	// Piccolo warm-up per garantire la corretta inizializzazione del socket su filesystem
	time.Sleep(400 * time.Millisecond)

	// Test di comunicazione sul canale isolato appena creato
	out, err := exec.Command("pactl", "info").CombinedOutput()
	if err != nil {
		log.Printf("Warning: Isolated PulseAudio connectivity check failed: %v.\nDetails: %s\nPulseAudio log: %s", 
			err, strings.TrimSpace(string(out)), strings.TrimSpace(pulseStderr.String()))
	} else {
		log.Println("Isolated PulseAudio instance successfully initialized and verified via loopback socket.")
	}

	return func() {
		log.Println("Stopping isolated PulseAudio and Xvfb background tasks...")
		if pulseCmd.Process != nil {
			_ = pulseCmd.Process.Kill()
		}
		if xvfbCmd.Process != nil {
			_ = xvfbCmd.Process.Kill()
		}
		_ = os.Remove(socketPath)
		_ = os.Remove(configPath)
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
	cleanupPulse := managePulseAudio(initialConfig)
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
