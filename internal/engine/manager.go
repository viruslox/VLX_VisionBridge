package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/user/VLX_VisionBridge/internal/db"
	"github.com/user/VLX_VisionBridge/internal/models"
)

type tailBuffer struct {
	buf []byte
	mu  sync.Mutex
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	const maxLen = 4096
	t.buf = append(t.buf, p...)
	if len(t.buf) > maxLen {
		t.buf = t.buf[len(t.buf)-maxLen:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

type RetryTracker struct {
	ConsecutiveCrashes int
	LastCrash          time.Time
}

type ProcessManager struct {
	cmd         *exec.Cmd
	config      *models.Config
	db          *sql.DB
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	cond        *sync.Cond
	isRunning   bool
	overlayCmds map[int]*exec.Cmd
	retries     map[string]*RetryTracker
}

func NewProcessManager(dbConn *sql.DB) *ProcessManager {
	pm := &ProcessManager{
		db:          dbConn,
		overlayCmds: make(map[int]*exec.Cmd),
		retries:     make(map[string]*RetryTracker),
	}
	pm.cond = sync.NewCond(&pm.mu)
	return pm
}

func (pm *ProcessManager) Start(ctx context.Context, config *models.Config) error {
	pm.mu.Lock()
	if pm.isRunning {
		pm.mu.Unlock()
		return fmt.Errorf("process already running")
	}

	pm.config = config
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	pm.isRunning = true
	pm.mu.Unlock()

	go pm.monitor()

	return nil
}

func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	if !pm.isRunning {
		pm.mu.Unlock()
		return
	}
	pm.isRunning = false

	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling GStreamer process to stop gracefully...")
		_ = pm.cmd.Process.Signal(syscall.SIGTERM)
	}

	if pm.cancel != nil {
		pm.cancel()
	}
	if pm.cond != nil {
		pm.cond.Broadcast()
	}

	for id, overlayCmd := range pm.overlayCmds {
		if overlayCmd != nil && overlayCmd.Process != nil {
			log.Printf("Signaling background process %d to stop...", id)
			_ = overlayCmd.Process.Signal(syscall.SIGTERM)
		}
	}

	pm.mu.Unlock()
}

func buildOverlayElement(id string, zIndex int, path string, width, height, x, y, volume *int) (string, string, string) {
	if path == "" {
		return "", "", ""
	}

	style := fmt.Sprintf("  #%s { z-index: %d; position: absolute; ", id, zIndex)
	if x != nil {
		style += fmt.Sprintf("left: %dpx; ", *x)
	} else {
		style += "left: 0; "
	}
	if y != nil {
		style += fmt.Sprintf("top: %dpx; ", *y)
	} else {
		style += "top: 0; "
	}
	if width != nil {
		style += fmt.Sprintf("width: %dpx; ", *width)
	}
	if height != nil {
		style += fmt.Sprintf("height: %dpx; ", *height)
	}
	style += "}\n"

	var element string
	lowerPath := strings.ToLower(path)

	srcURL := path
	if strings.HasPrefix(path, "/") {
		srcURL = "file://" + path
	}

	vol := 1.0
	if volume != nil {
		vol = float64(*volume) / 100.0
	}
	script := fmt.Sprintf("      var e_%s = document.getElementById('%s'); if (e_%s) e_%s.volume = %f;\n", id, id, id, id, vol)

	if strings.HasSuffix(lowerPath, ".mp4") || strings.HasSuffix(lowerPath, ".webm") {
		element = fmt.Sprintf(`  <video id="%s" src="%s" autoplay loop playsinline></video>`+"\n", id, srcURL)
	} else if strings.HasSuffix(lowerPath, ".png") || strings.HasSuffix(lowerPath, ".jpg") || strings.HasSuffix(lowerPath, ".jpeg") {
		element = fmt.Sprintf(`  <img id="%s" src="%s" />`+"\n", id, srcURL)
		script = ""
	} else if strings.HasSuffix(lowerPath, ".mp3") {
		element = fmt.Sprintf(`  <audio id="%s" src="%s" autoplay loop></audio>`+"\n", id, srcURL)
	} else {
		element = fmt.Sprintf(`  <iframe id="%s" src="%s" allow="camera; microphone; display-capture" allowtransparency="true" frameborder="0"></iframe>`+"\n", id, srcURL)
		script = ""
	}

	return style, element, script
}

// startEnvironment Inizializza Xvfb e PulseAudio prima del browser
func (pm *ProcessManager) startEnvironment(resWidth, resHeight string) {
	// Xvfb Process (ID 100)
	if _, exists := pm.overlayCmds[100]; !exists {
		xvfbCmd := exec.Command("Xvfb", ":99", "-screen", "0", fmt.Sprintf("%sx%sx24", resWidth, resHeight), "-ac", "-nolisten", "tcp")
		if err := xvfbCmd.Start(); err != nil {
			log.Printf("Xvfb might already be running: %v", err)
		} else {
			log.Printf("Started Xvfb on display :99 (%sx%s)", resWidth, resHeight)
			pm.overlayCmds[100] = xvfbCmd
		}
	}

	// PulseAudio Daemon
	pulseCmd := exec.Command("pulseaudio", "-D", "--exit-idle-time=-1")
	if err := pulseCmd.Run(); err == nil {
		log.Println("PulseAudio server initialized for software loopback audio.")
	}
}

func (pm *ProcessManager) manageOverlays(cfg *models.Config) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	activeOverlays := make(map[int]bool)
	activeOverlays[100] = true // Mantieni attivo Xvfb

	if cfg.Input.ChromiumSource.Active {
		activeOverlays[99] = true

		shouldStart := true
		if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
			if cmd.ProcessState == nil {
				shouldStart = false
			}
		}

		if shouldStart {
			cs := cfg.Input.ChromiumSource
			bgColor := cfg.Input.BgColor
			if bgColor == "" {
				bgColor = "black"
			}

			resParts := strings.Split(cfg.Input.Resolution, "x")
			resWidth := "1920"
			resHeight := "1080"
			if len(resParts) == 2 {
				resWidth = resParts[0]
				resHeight = resParts[1]
			}

			// Prepara Xvfb e PulseAudio
			pm.startEnvironment(resWidth, resHeight)

			// HTML Puro: Nessun Javascript astruso, nessun Canvas, nessun WebRTC.
			htmlContent := `<!DOCTYPE html>
<html>
<head>
<title>VisionBridge</title>
<style>
  * { margin: 0; padding: 0; overflow: hidden; }
  body { margin: 0; padding: 0; overflow: hidden; background: ` + bgColor + `; width: ` + resWidth + `px; height: ` + resHeight + `px; position: relative; }
  iframe, video, img { border: none; }
`
			var elements string
			var scripts string

			if cs.Z1Active {
				s, e, sc := buildOverlayElement("z1", 1, cs.Z1Path, cs.Z1Width, cs.Z1Height, cs.Z1X, cs.Z1Y, cs.Z1Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z2Active {
				s, e, sc := buildOverlayElement("z2", 2, cs.Z2Path, cs.Z2Width, cs.Z2Height, cs.Z2X, cs.Z2Y, cs.Z2Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z3Active {
				s, e, sc := buildOverlayElement("z3", 3, cs.Z3Path, cs.Z3Width, cs.Z3Height, cs.Z3X, cs.Z3Y, cs.Z3Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z4Active {
				s, e, sc := buildOverlayElement("z4", 4, cs.Z4Path, cs.Z4Width, cs.Z4Height, cs.Z4X, cs.Z4Y, cs.Z4Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z5Active {
				s, e, sc := buildOverlayElement("z5", 5, cs.Z5Path, cs.Z5Width, cs.Z5Height, cs.Z5X, cs.Z5Y, cs.Z5Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z6Active {
				s, e, sc := buildOverlayElement("z6", 6, cs.Z6Path, cs.Z6Width, cs.Z6Height, cs.Z6X, cs.Z6Y, cs.Z6Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z7Active {
				s, e, sc := buildOverlayElement("z7", 7, cs.Z7Path, cs.Z7Width, cs.Z7Height, cs.Z7X, cs.Z7Y, cs.Z7Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}
			if cs.Z8Active {
				s, e, sc := buildOverlayElement("z8", 8, cs.Z8Path, cs.Z8Width, cs.Z8Height, cs.Z8X, cs.Z8Y, cs.Z8Volume)
				htmlContent += s
				elements += e
				scripts += sc
			}

			htmlContent += `</style>
</head>
<body>
`
			htmlContent += elements
			if scripts != "" {
				htmlContent += "  <script>\n" + scripts + "  </script>\n"
			}
			htmlContent += `</body></html>`

			htmlPath := "/opt/VLX_VisionBridge/var/overlay.html"
			if err := os.MkdirAll("/opt/VLX_VisionBridge/var", 0755); err == nil {
				if writeErr := os.WriteFile(htmlPath, []byte(htmlContent), 0644); writeErr != nil {
					log.Printf("Failed to write overlay html: %v", writeErr)
				}
			}

			log.Printf("Starting Chromium browser natively on Xvfb Display :99")

			fileURL := htmlPath
			if strings.HasPrefix(htmlPath, "/") {
				fileURL = "file://" + htmlPath
			}

			chromeBin, err := exec.LookPath("chromium")
			if err != nil {
				chromeBin, err = exec.LookPath("chromium-browser")
				if err != nil {
					log.Printf("Failed to start Chromium overlay browser: chromium/chromium-browser not found")
					return
				}
			}

			// RIMOSSO L'HEADLESS MODE. Lo facciamo girare fisicamente su Xvfb (:99)
			cmd := exec.Command(chromeBin,
				"--kiosk",
				"--disable-infobars",
				"--disable-extensions",
				"--window-position=0,0",
				fmt.Sprintf("--window-size=%s,%s", resWidth, resHeight),
				"--autoplay-policy=no-user-gesture-required",
				"--disable-dev-shm-usage",
				"--no-sandbox",
				fileURL,
			)

			// Agganciamo Chromium al display virtuale e all'audio
			cmd.Env = append(os.Environ(), "DISPLAY=:99", "PULSE_SERVER=unix:/tmp/pulse-visionbridge/native")

			err = cmd.Start()
			if err != nil {
				log.Printf("Failed to start Chromium overlay browser: %v", err)
			} else {
				pm.overlayCmds[99] = cmd
				go pm.monitorChromium(cmd)
			}
		}
	}

	for id, cmd := range pm.overlayCmds {
		if !activeOverlays[id] {
			if cmd != nil && cmd.Process != nil {
				log.Printf("Stopping overlay browser for layer %d...", id)
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			delete(pm.overlayCmds, id)
		}
	}
}

func (pm *ProcessManager) monitorChromium(cmd *exec.Cmd) {
	err := cmd.Wait()

	pm.mu.Lock()
	if !pm.isRunning || pm.ctx.Err() != nil {
		pm.mu.Unlock()
		return
	}

	if currentCmd, exists := pm.overlayCmds[99]; !exists || currentCmd != cmd {
		pm.mu.Unlock()
		return
	}
	delete(pm.overlayCmds, 99)
	pm.mu.Unlock()

	log.Printf("Chromium overlay browser exited unexpectedly: %v", err)
	if pm.db != nil {
		_ = db.LogStreamEvent(pm.db, "crash", fmt.Sprintf("Chromium browser crashed: %v", err))
	}
}

func (pm *ProcessManager) UpdateFilter(config *models.Config) {
	pm.mu.Lock()
	pm.config = config
	pm.mu.Unlock()
}

func (pm *ProcessManager) UpdateConfig(config *models.Config) {
	pm.mu.Lock()
	pm.config = config

	if pm.config != nil && pm.config.Input.MediaSource.Active {
		var validLayers []models.Layer
		for _, layer := range pm.config.Input.MediaSource.Layers {
			if layer.ID >= 0 && layer.ID <= 2 {
				validLayers = append(validLayers, layer)
			}
		}
		if len(validLayers) > 3 {
			validLayers = validLayers[:3]
		}
		pm.config.Input.MediaSource.Layers = validLayers
	}

	if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
		log.Println("Signaling Chromium overlay process to stop gracefully for config update...")
		_ = cmd.Process.Signal(syscall.SIGTERM)
		delete(pm.overlayCmds, 99)
	}

	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling GStreamer process to stop gracefully for config update...")
		_ = pm.cmd.Process.Signal(syscall.SIGTERM)
	}
	if pm.cond != nil {
		pm.cond.Broadcast()
	}
	pm.mu.Unlock()
}

type monitorAction int

const (
	monitorActionStop monitorAction = iota
	monitorActionContinue
	monitorActionSleepConstant
	monitorActionSleepExponential
)

func (pm *ProcessManager) monitor() {
	backoff := 1 * time.Second
	var lastBuildErr string

	for {
		pm.mu.Lock()
		isActive := true
		if pm.config != nil {
			isActive = pm.config.Output.Active
		}
		pm.mu.Unlock()

		if !isActive {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		action, finalModule, isMisconfig := pm.executeSingleRun(&lastBuildErr)

		if isMisconfig && finalModule != "" {
			log.Printf("Misconfiguration detected for module %s, disabling it.", finalModule)
			pm.disableModule(finalModule)
			continue
		}

		if action == monitorActionSleepExponential && finalModule != "" {
			pm.mu.Lock()
			tracker, exists := pm.retries[finalModule]
			if !exists {
				tracker = &RetryTracker{}
				pm.retries[finalModule] = tracker
			}
			if !tracker.LastCrash.IsZero() && time.Since(tracker.LastCrash) > 30*time.Second {
				tracker.ConsecutiveCrashes = 0
			}
			tracker.ConsecutiveCrashes++
			tracker.LastCrash = time.Now()
			crashes := tracker.ConsecutiveCrashes
			pm.mu.Unlock()

			if crashes <= 5 {
				log.Printf("Module %s crashed %d times (quick retry in 1s)...", finalModule, crashes)
				time.Sleep(1 * time.Second)
				continue
			} else if crashes <= 7 {
				log.Printf("Module %s crashed %d times (wait retry in 10s)...", finalModule, crashes)
				time.Sleep(10 * time.Second)
				continue
			} else {
				log.Printf("Module %s crashed %d times. Max retries exceeded, disabling it.", finalModule, crashes)
				pm.disableModule(finalModule)
				continue
			}
		}

		switch action {
		case monitorActionStop:
			return
		case monitorActionContinue:
			continue
		case monitorActionSleepConstant:
			time.Sleep(backoff)
			continue
		case monitorActionSleepExponential:
			log.Printf("Restarting GStreamer in %v...", backoff)
			time.Sleep(backoff)

			backoff *= 2
			maxBackoff := 30 * time.Second
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (pm *ProcessManager) disableModule(module string) {
	if module == "" || pm.config == nil {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if module == "[chromium]" {
		pm.config.Input.ChromiumSource.Active = false
	} else if strings.HasPrefix(module, "[layer ") {
		parts := strings.Split(module, "]")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "[layer ")
			if id, err := strconv.Atoi(idStr); err == nil {
				for i, layer := range pm.config.Input.MediaSource.Layers {
					if layer.ID == id {
						pm.config.Input.MediaSource.Layers[i].Active = false
						break
					}
				}
			}
		}
	} else if module == "[media_source]" {
		pm.config.Input.MediaSource.Active = false
	}
}

func identifyErrorModule(stderr string, cfg *models.Config) string {
	if cfg == nil || stderr == "" {
		return ""
	}

	if cfg.Input.MediaSource.Active {
		for _, layer := range cfg.Input.MediaSource.Layers {
			if !layer.Active || layer.InputPath == "" {
				continue
			}
			if strings.Contains(stderr, layer.InputPath) {
				return fmt.Sprintf("[layer %d] [input]", layer.ID)
			}
		}
	}

	for _, dest := range cfg.Output.Destinations {
		if dest == "" {
			continue
		}
		if strings.Contains(stderr, dest) {
			return "[output]"
		}
	}
	return ""
}

func (pm *ProcessManager) executeSingleRun(lastBuildErr *string) (monitorAction, string, bool) {
	pm.mu.Lock()
	ctx := pm.ctx
	cfg := pm.config
	isRunning := pm.isRunning
	pm.mu.Unlock()

	if !isRunning || ctx.Err() != nil {
		return monitorActionStop, "", false
	}

	pm.manageOverlays(cfg)

	args, err := BuildPipelineArgs(cfg)
	if err != nil {
		return monitorActionSleepConstant, "", false
	}

	if len(args) == 0 {
		pm.mu.Lock()
		if pm.isRunning && pm.ctx.Err() == nil {
			pm.cond.Wait()
		}
		pm.mu.Unlock()
		return monitorActionContinue, "", false
	}

	started, runErr, stderrStr := pm.runProcess(ctx, args)
	if !started {
		return monitorActionSleepConstant, "", false
	}

	if ctx.Err() != nil {
		return monitorActionStop, "", false
	}

	errMsg := "GStreamer exited unexpectedly"
	var finalModule string
	var isMisconfig bool

	if runErr != nil {
		if stderrStr != "" {
			lines := strings.Split(strings.TrimSpace(stderrStr), "\n")
			if len(lines) > 0 {
				isMisconfig = true // GStreamer crashes are typically pipeline config errors
			}
		}
		errMsg = fmt.Sprintf("GStreamer crashed: %v", runErr)
	}
	log.Println(errMsg)

	return monitorActionSleepExponential, finalModule, isMisconfig
}

func (pm *ProcessManager) runProcess(ctx context.Context, gstArgs []string) (bool, error, string) {
	gstCmd := exec.Command("gst-launch-1.0", gstArgs...)

	tb := &tailBuffer{}
	gstCmd.Stderr = tb

	pm.mu.Lock()
	pm.cmd = gstCmd
	pm.mu.Unlock()

	defer func() {
		pm.mu.Lock()
		pm.cmd = nil
		pm.mu.Unlock()
	}()

	log.Println("Starting GStreamer native X11 pipeline... (Streaming to MediaMTX)")

	err := gstCmd.Start()
	if err != nil {
		return false, err, tb.String()
	}

	errWait, stderrStr := pm.waitForProcess(ctx, gstCmd)
	return true, errWait, stderrStr
}

func (pm *ProcessManager) waitForProcess(ctx context.Context, cmd *exec.Cmd) (error, string) {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var runErr error
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		runErr = <-done
	case runErr = <-done:
	}

	stderrStr := ""
	if tb, ok := cmd.Stderr.(*tailBuffer); ok {
		stderrStr = tb.String()
	}

	return runErr, stderrStr
}

func (pm *ProcessManager) ReloadChromium() {
	pm.mu.Lock()
	if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		delete(pm.overlayCmds, 99)
	}
	cfg := pm.config
	pm.mu.Unlock()

	if cfg != nil {
		pm.manageOverlays(cfg)
	}
}
