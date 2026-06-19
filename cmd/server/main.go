package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
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
func managePulseAudio(cfg *models.Config) func() {
	var dbusPid int

	// Helper inline per catturare l'output combinato (stdout + stderr) in caso di errore
	runAudioCmd := func(name string, arg ...string) error {
		cmd := exec.Command(name, arg...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("exit status %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// 1. Controllo preventivo: verifichiamo se il server audio (Pulse o PipeWire) è già raggiungibile nativamente
	pulseRunning := exec.Command("pactl", "info").Run() == nil
	if pulseRunning {
		log.Println("PulseAudio/PipeWire daemon is already running and accessible natively.")
	} else {
		log.Println("Native PulseAudio/PipeWire daemon not reachable. Checking session context...")
	}

	// 2. Inizializza D-BUS solo se non è già presente nell'ambiente corrente
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		out, err := exec.Command("dbus-launch").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					os.Setenv(parts[0], parts[1])
					if parts[0] == "DBUS_SESSION_BUS_PID" {
						pid, err := strconv.Atoi(parts[1])
						if err == nil {
							dbusPid = pid
						}
					}
				}
			}
			log.Println("Launched a new isolated D-Bus session bus.")
			// Se prima non era raggiungibile, ricontrolliamo sotto il nuovo contesto bus
			if !pulseRunning {
				pulseRunning = exec.Command("pactl", "info").Run() == nil
			}
		} else {
			log.Printf("Warning: Failed to launch dbus: %v", err)
		}
	}

	// 3. Determina e applica la risoluzione dinamica per Xvfb
	screenRes := "1920x1080" // Fallback
	if cfg != nil && cfg.Input.Resolution != "" {
		screenRes = cfg.Input.Resolution
	}

	log.Printf("Starting Xvfb on display :99 with resolution %s...", screenRes)
	_ = exec.Command("Xvfb", ":99", "-screen", "0", fmt.Sprintf("%sx24", screenRes)).Start()
	os.Setenv("DISPLAY", ":99")

	// 4. Se il server audio non è ancora accessibile, tenta l'avvio del demone autonomo
	if !pulseRunning {
		log.Println("Attempting to start standalone PulseAudio daemon...")
		_ = exec.Command("pulseaudio", "-k").Run()
		if err := runAudioCmd("pulseaudio", "--start", "--exit-idle-time=-1"); err != nil {
			log.Printf("Warning: Failed to start PulseAudio daemon: %v", err)
		} else {
			log.Println("PulseAudio daemon started successfully.")
			pulseRunning = true
		}
	}

	// 5. Creazione della scheda audio virtuale null-sink con logging dettagliato dello stderr
	_ = exec.Command("pactl", "unload-module", "module-null-sink").Run()
	if err := runAudioCmd("pactl", "load-module", "module-null-sink", "sink_name=vlx_chromium_sink", "sink_properties=device.description=vlx_chromium_sink"); err != nil {
		log.Printf("Warning: Failed to create virtual audio card: %v", err)
	} else {
		log.Println("Virtual audio card 'vlx_chromium_sink' created successfully.")
	}

	// 6. Impostazione del sink predefinito con logging dello stderr
	if err := runAudioCmd("pactl", "set-default-sink", "vlx_chromium_sink"); err != nil {
		log.Printf("Warning: Failed to set default sink: %v", err)
	} else {
		log.Println("Set 'vlx_chromium_sink' as default audio sink.")
	}

	return func() {
		// Chiudiamo il demone pulseaudio solo se lo abbiamo spawnato noi direttamente
		if !pulseRunning {
			log.Println("Stopping PulseAudio daemon...")
			_ = exec.Command("pulseaudio", "-k").Run()
		}
		if dbusPid > 0 {
			_ = syscall.Kill(dbusPid, syscall.SIGTERM)
		}
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
