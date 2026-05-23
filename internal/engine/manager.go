package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"strings"
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

// ProcessManager manages the FFmpeg process.
type ProcessManager struct {
	cmd       *exec.Cmd
	config    *models.Config
	db        *sql.DB
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	cond      *sync.Cond
	isRunning bool
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(dbConn *sql.DB) *ProcessManager {
	pm := &ProcessManager{
		db: dbConn,
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
	// Avoid returning a cancel func since we don't use it directly, instead we signal via channel
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	pm.isRunning = true
	pm.mu.Unlock()

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

	// Issue graceful signal if command is running
	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling FFmpeg process to stop gracefully...")
		_ = pm.cmd.Process.Signal(syscall.SIGTERM)

		// Create a separate wait path since run wait does not complete if process hangs
		// But we don't call Wait() directly here to avoid race with cmd.Run().
		// We rely on cmd.Run() returning in monitor() and monitor handling the cancellation.
	}

	// Trigger cancellation
	if pm.cancel != nil {
		pm.cancel()
	}
	if pm.cond != nil {
		pm.cond.Broadcast()
	}
	pm.mu.Unlock()
}

// UpdateConfig updates the configuration and signals the monitor loop.
func (pm *ProcessManager) UpdateConfig(config *models.Config) {
	pm.mu.Lock()
	pm.config = config

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

// monitor runs the FFmpeg process and handles automatic recovery.
func (pm *ProcessManager) monitor() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	var lastBuildErr string

	for {
		action := pm.executeSingleRun(&lastBuildErr)

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

			// Exponential backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func identifyErrorModule(stderr string, cfg *models.Config) string {
	if cfg == nil || stderr == "" {
		return ""
	}

	for _, layer := range cfg.Layers {
		if !layer.Active || layer.InputPath == "" {
			continue
		}
		if strings.Contains(stderr, layer.InputPath) {
			return fmt.Sprintf("[layer %d] [input]", layer.ID)
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

func (pm *ProcessManager) executeSingleRun(lastBuildErr *string) monitorAction {
	pm.mu.Lock()
	ctx := pm.ctx
	cfg := pm.config
	isRunning := pm.isRunning
	pm.mu.Unlock()

	if !isRunning {
		log.Println("Process manager shutting down gracefully")
		return monitorActionStop
	}

	if ctx.Err() != nil {
		log.Println("Process manager shutting down gracefully (context canceled)")
		return monitorActionStop
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		errMsg := fmt.Sprintf("Build args failed: %v", err)
		log.Printf("Failed to build FFmpeg args: %v", err)
		if pm.db != nil && *lastBuildErr != errMsg {
			_ = db.LogStreamEvent(pm.db, "error", errMsg)
			*lastBuildErr = errMsg
		}
		return monitorActionSleepConstant
	}
	// Reset the error cache when the build is successful so a future identical error can be logged again
	*lastBuildErr = ""

	if len(args) == 0 {
		log.Println("No active layers, not starting FFmpeg.")

		pm.mu.Lock()
		// We must check the condition inside the lock to avoid lost wakeups.
		// Re-evaluating len(args) here would require rebuilding args, which we don't want to do inside the lock.
		// Instead, we just wait until we are signaled by UpdateConfig or Stop.
		if pm.isRunning && pm.ctx.Err() == nil {
			pm.cond.Wait()
		}
		pm.mu.Unlock()
		return monitorActionContinue
	}

	started, runErr, stderrStr := pm.runProcess(ctx, args)
	if !started {
		log.Printf("Failed to start FFmpeg: %v", runErr)
		return monitorActionSleepConstant
	}

	if ctx.Err() != nil {
		// Context cancelled, normal shutdown
		if pm.db != nil {
			_ = db.LogStreamEvent(pm.db, "stop", "FFmpeg process stopped gracefully")
		}
		return monitorActionStop
	}

	// Unexpected exit
	errMsg := "FFmpeg exited unexpectedly"
	if runErr != nil {
		reason := ""
		if stderrStr != "" {
			lines := strings.Split(strings.TrimSpace(stderrStr), "\n")
			if len(lines) > 0 {
				// Find the last actual error line, "Conversion failed!" is often just the generic exit message
				reason = lines[len(lines)-1]
				if reason == "Conversion failed!" && len(lines) > 1 {
					reason = lines[len(lines)-2] + " | " + reason
				}
				if len(lines) > 10 {
					log.Printf("FFmpeg stderr tail:\n%s", strings.Join(lines[len(lines)-30:], "\n"))
				} else {
					log.Printf("FFmpeg stderr tail:\n%s", stderrStr)
				}
				modulePrefix := identifyErrorModule(stderrStr, cfg)
				if modulePrefix != "" {
					reason = modulePrefix + " " + reason
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

	return monitorActionSleepExponential
}

func (pm *ProcessManager) runProcess(ctx context.Context, args []string) (bool, error, string) {
	// Create command without context to allow graceful SIGTERM before context kill
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

	// Start the process asynchronously to allow monitoring for context cancellation
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
		// Process exited on its own
	}

	stderrStr := ""
	if tb, ok := cmd.Stderr.(*tailBuffer); ok {
		stderrStr = tb.String()
	}

	return runErr, stderrStr
}
