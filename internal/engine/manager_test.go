package engine

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestProcessManager_Start_AlreadyRunning(t *testing.T) {
	pm := NewProcessManager(nil)
	pm.isRunning = true

	err := pm.Start(context.Background(), &models.Config{Input: models.InputSettings{Resolution: "1920x1080"}})
	if err == nil {
		t.Errorf("Expected error when starting already running process manager")
	} else if err.Error() != "process already running" {
		t.Errorf("Expected 'process already running' error, got: %v", err)
	}
}

func TestProcessManager_Start_Success(t *testing.T) {
	pm := NewProcessManager(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &models.Config{
		Input: models.InputSettings{Resolution: "1920x1080"},
	}

	err := pm.Start(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	pm.mu.Lock()
	isRunning := pm.isRunning
	pm.mu.Unlock()

	if !isRunning {
		t.Errorf("Expected isRunning to be true")
	}

	// Wait briefly to allow goroutine to start
	time.Sleep(10 * time.Millisecond)

	pm.Stop()
}

func TestProcessManager_Stop_NotRunning(t *testing.T) {
	pm := NewProcessManager(nil)
	// Default state is not running
	pm.Stop()
	if pm.isRunning {
		t.Errorf("Expected isRunning to be false")
	}
}

func TestProcessManager_Stop_RunningNoCmd(t *testing.T) {
	pm := NewProcessManager(nil)
	pm.isRunning = true
	ctx, cancel := context.WithCancel(context.Background())
	pm.cancel = cancel
	pm.ctx = ctx

	pm.Stop()

	if pm.isRunning {
		t.Errorf("Expected isRunning to be false")
	}

	if ctx.Err() == nil {
		t.Errorf("Expected context to be cancelled")
	}
}

func TestProcessManager_Stop_RunningWithCmd(t *testing.T) {
	pm := NewProcessManager(nil)
	pm.isRunning = true
	ctx, cancel := context.WithCancel(context.Background())
	pm.cancel = cancel
	pm.ctx = ctx

	// Start a dummy command
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dummy command: %v", err)
	}
	pm.cmd = cmd

	pm.Stop()

	if pm.isRunning {
		t.Errorf("Expected isRunning to be false")
	}

	if ctx.Err() == nil {
		t.Errorf("Expected context to be cancelled")
	}

	err := cmd.Wait()
	if err == nil {
		t.Errorf("Expected command to be terminated, but it exited normally")
	}
}

func TestProcessManager_UpdateConfig(t *testing.T) {
	pm := NewProcessManager(nil)

	// Test update config when no process is running
	config := &models.Config{
		Input: models.InputSettings{Resolution: "1920x1080"},
		Output: models.OutputSettings{
			Resolution: "1920x1080",
		},
	}
	pm.UpdateConfig(config)

	pm.mu.Lock()
	if pm.config != config {
		t.Errorf("Expected config to be updated")
	}
	pm.mu.Unlock()

	// Test condition variable broadcasting
	condUnblocked := make(chan struct{})
	go func() {
		pm.mu.Lock()
		pm.cond.Wait()
		pm.mu.Unlock()
		close(condUnblocked)
	}()

	// Allow goroutine to start waiting
	time.Sleep(10 * time.Millisecond)

	// Start a dummy command to simulate a running process
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dummy command: %v", err)
	}

	pm.mu.Lock()
	pm.cmd = cmd
	pm.mu.Unlock()

	newConfig := &models.Config{
		Input: models.InputSettings{Resolution: "1920x1080"},
		Output: models.OutputSettings{
			Resolution: "1280x720",
		},
	}

	pm.UpdateConfig(newConfig)

	pm.mu.Lock()
	if pm.config != newConfig {
		t.Errorf("Expected config to be updated to newConfig")
	}
	pm.mu.Unlock()

	// Verify condition variable unblocked
	select {
	case <-condUnblocked:
		// Success
	case <-time.After(1 * time.Second):
		t.Errorf("Condition variable was not broadcasted")
	}

	// Verify the process was signaled to stop
	err := cmd.Wait()
	if err == nil {
		t.Errorf("Expected command to be terminated by signal, but it exited normally")
	}
}
