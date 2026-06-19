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

	"github.com/go-zeromq/zmq4"
	"github.com/user/VLX_VisionBridge/internal/db"
	"github.com/user/VLX_VisionBridge/internal/engine/source"
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

// ProcessManager manages the FFmpeg process.
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

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(dbConn *sql.DB) *ProcessManager {
	pm := &ProcessManager{
		db:          dbConn,
		overlayCmds: make(map[int]*exec.Cmd),
		retries:     make(map[string]*RetryTracker),
	}
	pm.cond = sync.NewCond(&pm.mu)
	return pm
}

// Start starts the FFmpeg process and monitors it.
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

	go pm.StartConnectorListener()
	go pm.monitor()

	return nil
}

// Stop gracefully stops the FFmpeg process.
func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	if !pm.isRunning {
		pm.mu.Unlock()
		return
	}
	pm.isRunning = false

	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling FFmpeg process to stop gracefully...")
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
			log.Printf("Signaling overlay process for layer %d to stop gracefully...", id)
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
		element = fmt.Sprintf(`  <video id="%s" src="%s" autoplay loop></video>`+"\n", id, srcURL)
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

func (pm *ProcessManager) manageOverlays(cfg *models.Config) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	activeOverlays := make(map[int]bool)

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

			htmlContent := `<!DOCTYPE html>
<html>
<head>
<style>
  * { margin: 0; padding: 0; overflow: hidden; }
  body { margin: 0; padding: 0; overflow: hidden; background: ` + bgColor + `; }
  iframe, video { margin: 0; padding: 0; border: none; }
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
<body style="margin: 0; padding: 0; width: ` + resWidth + `px; height: ` + resHeight + `px;">
`
			htmlContent += elements
			if scripts != "" {
				htmlContent += "  <script>\n" + scripts + "  </script>\n"
			}
			htmlContent += `</body>
</html>`

			htmlPath := "/opt/VLX_VisionBridge/var/overlay.html"
			if err := os.MkdirAll("/opt/VLX_VisionBridge/var", 0755); err == nil {
				if writeErr := os.WriteFile(htmlPath, []byte(htmlContent), 0644); writeErr != nil {
					log.Printf("Failed to write overlay html: %v", writeErr)
				}
			}

			log.Printf("Starting Chromium overlay browser with generated HTML")

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

			// OPTIMIZATION: Aggiunti flag per disattivare qualsiasi throttling grafico ed energetico di Chromium headless
			cmd := exec.Command(chromeBin, 
				"--kiosk", 
				"--disable-infobars", 
				"--disable-extensions", 
				"--test-type",
				fmt.Sprintf("--window-size=%s,%s", resWidth, resHeight), 
				"--window-position=0,0", 
				"--hide-scrollbars", 
				"--no-sandbox", 
				"--disable-dev-shm-usage",
				"--autoplay-policy=no-user-gesture-required", 
				"--force-device-scale-factor=1",
				"--disable-background-timer-throttling",
				"--disable-backgrounding-occluded-windows",
				"--disable-renderer-backgrounding",
				"--unthrottled-timer-nested-iframes",
				"--disable-frame-rate-limit",
				"--use-gl=swiftshader",
				fileURL,
			)

			cmd.Env = append(os.Environ(), "DISPLAY=:99", "PULSE_SINK=vlx_chromium_sink")

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

	if config != nil && config.Input.FFmpegSource.Active {
		var validLayers []models.Layer
		for _, layer := range config.Input.FFmpegSource.Layers {
			if layer.ID >= 0 && layer.ID <= 2 {
				validLayers = append(validLayers, layer)
			}
		}
		if len(validLayers) > 3 {
			validLayers = validLayers[:3]
		}
		config.Input.FFmpegSource.Layers = validLayers
	}

	req := zmq4.NewReq(context.Background())
	defer req.Close()
	err := req.Dial("tcp://127.0.0.1:5555")
	if err != nil {
		log.Printf("ZMQ dial error: %v", err)
		return
	}

	if config.Input.FFmpegSource.Active {
		for _, layer := range config.Input.FFmpegSource.Layers {
			if layer.ID == 99 {
				continue
			}

			res := source.BuildInputArgs(layer)
			media := layer.Media
			if media == "" {
				media = "Video+Audio"
			}

			if !layer.Active {
				if (media == "Video" || media == "Video+Audio") && res.HasVideo {
					sendZMQCommand(req, fmt.Sprintf("overlay@layer%d x -9999", layer.ID))
				}
				if (media == "Audio" || media == "Video+Audio") && res.HasAudio {
					sendZMQCommand(req, fmt.Sprintf("volume@layer%d volume 0.0", layer.ID))
				}
				continue
			}

			if (media == "Video" || media == "Video+Audio") && res.HasVideo {
				sendZMQCommand(req, fmt.Sprintf("overlay@layer%d x %d", layer.ID, layer.X))
				sendZMQCommand(req, fmt.Sprintf("overlay@layer%d y %d", layer.ID, layer.Y))
			}

			if (media == "Audio" || media == "Video+Audio") && res.HasAudio {
				vol := 1.0
				if layer.Volume != nil {
					vol = float64(*layer.Volume) / 100.0
				}
				sendZMQCommand(req, fmt.Sprintf("volume@layer%d volume %f", layer.ID, vol))
			}
		}
	}
}

func sendZMQCommand(req zmq4.Socket, cmd string) {
	err := req.Send(zmq4.NewMsgString(cmd))
	if err != nil {
		log.Printf("ZMQ send error for '%s': %v", cmd, err)
		return
	}
	reply, err := req.Recv()
	if err != nil {
		log.Printf("ZMQ recv error for '%s': %v", cmd, err)
		return
	}
	log.Printf("ZMQ reply for '%s': %s", cmd, string(reply.Frames[0]))
}

func (pm *ProcessManager) UpdateConfig(config *models.Config) {
	pm.mu.Lock()
	pm.config = config

	if pm.config != nil && pm.config.Input.FFmpegSource.Active {
		var validLayers []models.Layer
		for _, layer := range pm.config.Input.FFmpegSource.Layers {
			if layer.ID >= 0 && layer.ID <= 2 {
				validLayers = append(validLayers, layer)
			}
		}
		if len(validLayers) > 3 {
			validLayers = validLayers[:3]
		}
		pm.config.Input.FFmpegSource.Layers = validLayers
	}

	if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
		log.Println("Signaling Chromium overlay process to stop gracefully for config update...")
		_ = cmd.Process.Signal(syscall.SIGTERM)
		delete(pm.overlayCmds, 99)
	}

	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling FFmpeg process to stop gracefully for config update...")
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
			log.Printf("Restarting FFmpeg in %v...", backoff)
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

	log.Printf("Disabling misconfigured/failing module: %s", module)

	if module == "[chromium]" {
		pm.config.Input.ChromiumSource.Active = false
	} else if strings.HasPrefix(module, "[layer ") {
		parts := strings.Split(module, "]")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "[layer ")
			if id, err := strconv.Atoi(idStr); err == nil {
				for i, layer := range pm.config.Input.FFmpegSource.Layers {
					if layer.ID == id {
						pm.config.Input.FFmpegSource.Layers[i].Active = false
						break
					}
				}
			}
		}
	} else if module == "[ffmpeg_source]" {
		pm.config.Input.FFmpegSource.Active = false
	}
}

func identifyErrorModule(stderr string, cfg *models.Config) string {
	if cfg == nil || stderr == "" {
		return ""
	}

	if cfg.Input.FFmpegSource.Active {
		for _, layer := range cfg.Input.FFmpegSource.Layers {
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

	lowerStderr := strings.ToLower(stderr)
	if strings.Contains(lowerStderr, "filter") || strings.Contains(lowerStderr, "parsed_") || strings.Contains(lowerStderr, "overlay") || strings.Contains(lowerStderr, "scale=") {
		return "[mixer]"
	}

	return ""
}

func (pm *ProcessManager) executeSingleRun(lastBuildErr *string) (monitorAction, string, bool) {
	pm.mu.Lock()
	ctx := pm.ctx
	cfg := pm.config
	isRunning := pm.isRunning
	pm.mu.Unlock()

	if !isRunning {
		log.Println("Process manager shutting down gracefully")
		return monitorActionStop, "", false
	}

	if ctx.Err() != nil {
		log.Println("Process manager shutting down gracefully (context canceled)")
		return monitorActionStop, "", false
	}

	pm.manageOverlays(cfg)

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		errMsg := fmt.Sprintf("Build args failed: %v", err)
		log.Printf("Failed to build FFmpeg args: %v", err)
		if pm.db != nil && *lastBuildErr != errMsg {
			_ = db.LogStreamEvent(pm.db, "error", errMsg)
			*lastBuildErr = errMsg
		}
		return monitorActionSleepConstant, "", false
	}
	*lastBuildErr = ""

	if len(args) == 0 {
		log.Println("No active layers, not starting FFmpeg.")

		pm.mu.Lock()
		if pm.isRunning && pm.ctx.Err() == nil {
			pm.cond.Wait()
		}
		pm.mu.Unlock()
		return monitorActionContinue, "", false
	}

	started, runErr, stderrStr := pm.runProcess(ctx, args)
	if !started {
		log.Printf("Failed to start FFmpeg: %v", runErr)
		return monitorActionSleepConstant, "", false
	}

	if ctx.Err() != nil {
		if pm.db != nil {
			_ = db.LogStreamEvent(pm.db, "stop", "FFmpeg process stopped gracefully")
		}
		return monitorActionStop, "", false
	}

	errMsg := "FFmpeg exited unexpectedly"
	var finalModule string
	var isMisconfig bool

	if runErr != nil {
		reason := ""
		if stderrStr != "" {
			lines := strings.Split(strings.TrimSpace(stderrStr), "\n")
			if len(lines) > 0 {
				reason = lines[len(lines)-1]
				if reason == "Conversion failed!" && len(lines) > 1 {
					reason = lines[len(lines)-2] + " | " + reason
				}
				if len(lines) > 10 {
					startIdx := len(lines) - 30
					if startIdx < 0 {
						startIdx = 0
					}
					log.Printf("FFmpeg stderr tail:\n%s", strings.Join(lines[startIdx:], "\n"))
				} else {
					log.Printf("FFmpeg stderr tail:\n%s", stderrStr)
				}

				modulePrefix := identifyErrorModule(stderrStr, cfg)
				finalModule = modulePrefix
				if modulePrefix != "" {
					reason = modulePrefix + " " + reason
				}

				lowerStderr := strings.ToLower(stderrStr)
				if strings.Contains(lowerStderr, "no such file or directory") || strings.Contains(lowerStderr, "invalid argument") || strings.Contains(lowerStderr, "could not find codec parameters") || strings.Contains(lowerStderr, "not found") {
					isMisconfig = true
				}
			}
		}
		if reason != "" {
			errMsg = fmt.Sprintf("FFmpeg crashed: %v, reason: %s", runErr, reason)
		} else {
			errMsg = fmt.Sprintf("FFmpeg crashed: %v", runErr)
		}
	}
	log.Println(errMsg)
	if pm.db != nil {
		_ = db.LogStreamEvent(pm.db, "crash", errMsg)
	}

	return monitorActionSleepExponential, finalModule, isMisconfig
}

func (pm *ProcessManager) runProcess(ctx context.Context, args []string) (bool, error, string) {
	cmd := exec.Command("ffmpeg", args...)

	tb := &tailBuffer{}
	cmd.Stderr = tb

	pm.mu.Lock()
	pm.cmd = cmd
	pm.mu.Unlock()

	defer func() {
		pm.mu.Lock()
		pm.cmd = nil
		pm.mu.Unlock()
	}()

	if pm.db != nil {
		_ = db.LogStreamEvent(pm.db, "start", "Starting FFmpeg process")
	}
	log.Println("Starting FFmpeg process...")

	err := cmd.Start()
	if err != nil {
		return false, err, tb.String()
	}

	errWait, stderrStr := pm.waitForProcess(ctx, cmd)
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
		log.Println("Context cancelled, waiting for FFmpeg to stop...")
		select {
		case <-time.After(5 * time.Second):
			log.Println("FFmpeg process did not stop gracefully, killing it...")
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			runErr = <-done
		case runErr = <-done:
			log.Println("FFmpeg process stopped gracefully.")
		}
	case runErr = <-done:
	}

	stderrStr := ""
	if tb, ok := cmd.Stderr.(*tailBuffer); ok {
		stderrStr = tb.String()
	}

	return runErr, stderrStr
}

// ReloadChromium safely restarts the Chromium headless process.
func (pm *ProcessManager) ReloadChromium() {
	pm.mu.Lock()
	if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
		log.Println("Signaling Chromium overlay process to stop gracefully for manual reload...")
		_ = cmd.Process.Signal(syscall.SIGTERM)
		delete(pm.overlayCmds, 99)
	}
	cfg := pm.config
	pm.mu.Unlock()

	if cfg != nil {
		pm.manageOverlays(cfg)
	}
}
