package engine

import (
	"context"
	"os/exec"
	"testing"
)

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
