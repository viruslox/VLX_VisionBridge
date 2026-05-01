package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/user/go-live-orchestrator/internal/db"
	"github.com/user/go-live-orchestrator/internal/models"
)

// ProcessManager manages the FFmpeg process.
type ProcessManager struct {
	cmd       *exec.Cmd
	config    *models.Config
	db        *sql.DB
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	isRunning bool
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(dbConn *sql.DB) *ProcessManager {
	return &ProcessManager{
		db: dbConn,
	}
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
	pm.mu.Unlock()
}

// monitor runs the FFmpeg process and handles automatic recovery.
func (pm *ProcessManager) monitor() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		pm.mu.Lock()
		ctx := pm.ctx
		cfg := pm.config
		isRunning := pm.isRunning
		pm.mu.Unlock()

		if !isRunning {
			log.Println("Process manager shutting down gracefully")
			return
		}

		if ctx.Err() != nil {
			log.Println("Process manager shutting down gracefully (context canceled)")
			return
		}

		args, err := BuildFFmpegArgs(cfg)
		if err != nil {
			log.Printf("Failed to build FFmpeg args: %v", err)
			if pm.db != nil {
				_ = db.LogStreamEvent(pm.db, "error", fmt.Sprintf("Build args failed: %v", err))
			}
			time.Sleep(backoff)
			continue
		}

		if len(args) == 0 {
			log.Println("No active layers, not starting FFmpeg.")
			time.Sleep(5 * time.Second) // wait before checking again, perhaps wait on a condition variable in the future
			continue
		}

		// Create command without context to allow graceful SIGTERM before context kill
		cmd := exec.Command("ffmpeg", args...)

		pm.mu.Lock()
		pm.cmd = cmd
		pm.mu.Unlock()

		if pm.db != nil {
			_ = db.LogStreamEvent(pm.db, "start", "Starting FFmpeg process")
		}
		log.Println("Starting FFmpeg process...")

		// Start the process asynchronously to allow monitoring for context cancellation
		err = cmd.Start()
		if err != nil {
			log.Printf("Failed to start FFmpeg: %v", err)
			time.Sleep(backoff)
			continue
		}

		// Monitor context cancellation and kill process if graceful shutdown takes too long
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
			// We break out of the loop next time
		case runErr = <-done:
			// Process exited on its own
		}

		pm.mu.Lock()
		pm.cmd = nil
		pm.mu.Unlock()

		if ctx.Err() != nil {
			// Context cancelled, normal shutdown
			if pm.db != nil {
				_ = db.LogStreamEvent(pm.db, "stop", "FFmpeg process stopped gracefully")
			}
			return
		}

		// Unexpected exit
		errMsg := "FFmpeg exited unexpectedly"
		if runErr != nil {
			errMsg = fmt.Sprintf("FFmpeg crashed: %v", runErr)
		}
		log.Println(errMsg)
		if pm.db != nil {
			_ = db.LogStreamEvent(pm.db, "crash", errMsg)
		}

		log.Printf("Restarting FFmpeg in %v...", backoff)
		time.Sleep(backoff)

		// Exponential backoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
