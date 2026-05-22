package main

import (
	"errors"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/config"
	"github.com/user/VLX_VisionBridge/internal/models"
)

// MockProcessUpdater is a mock implementation of ProcessUpdater
type MockProcessUpdater struct {
	updateConfigCalled bool
	lastConfig         *models.Config
}

func (m *MockProcessUpdater) UpdateConfig(config *models.Config) {
	m.updateConfigCalled = true
	m.lastConfig = config
}

func TestCheckEUID(t *testing.T) {
	if err := CheckEUID(0); err == nil {
		t.Errorf("Expected error for euid=0 (root), got nil")
	}

	if err := CheckEUID(1000); err != nil {
		t.Errorf("Expected no error for euid!=0, got %v", err)
	}
}

func TestCheckFFmpeg(t *testing.T) {
	mockLookPathFound := func(file string) (string, error) {
		return "/usr/bin/ffmpeg", nil
	}

	if err := CheckFFmpeg(mockLookPathFound); err != nil {
		t.Errorf("Expected no error when ffmpeg is found, got %v", err)
	}

	mockLookPathNotFound := func(file string) (string, error) {
		return "", errors.New("not found")
	}

	if err := CheckFFmpeg(mockLookPathNotFound); err == nil {
		t.Errorf("Expected error when ffmpeg is not found, got nil")
	}
}

func TestResolveConfigPath(t *testing.T) {
	if path := ResolveConfigPath("/custom/visionbridge.settings"); path != "/custom/visionbridge.settings" {
		t.Errorf("Expected /custom/visionbridge.settings, got %s", path)
	}

	// It's hard to predict if configs/visionbridge.settings exists in the test environment
	// so we test only the fallback logic when env is empty
	path := ResolveConfigPath("")
	if path != "configs/visionbridge.settings" && path != "/opt/VLX_VisionBridge/etc/visionbridge.settings" {
		t.Errorf("Expected configs/visionbridge.settings or /opt/VLX_VisionBridge/etc/visionbridge.settings, got %s", path)
	}
}

func TestResolveDSN(t *testing.T) {
	envDSN := "postgres://env:pass@localhost:5432/db"
	configDSN := "postgres://config:pass@localhost:5432/db"

	if dsn := ResolveDSN(envDSN, configDSN); dsn != envDSN {
		t.Errorf("Expected %s, got %s", envDSN, dsn)
	}

	if dsn := ResolveDSN("", configDSN); dsn != configDSN {
		t.Errorf("Expected %s, got %s", configDSN, dsn)
	}
}

func TestHandleConfigChange(t *testing.T) {
	mockPM := &MockProcessUpdater{}
	newCfg := &models.Config{}

	// Test case: RequiresRestart = true
	diffRestart := config.DiffResult{RequiresRestart: true, RequiresFilterUpdate: false}
	HandleConfigChange(mockPM, newCfg, diffRestart)
	if !mockPM.updateConfigCalled || mockPM.lastConfig != newCfg {
		t.Errorf("Expected UpdateConfig to be called with newCfg for RequiresRestart")
	}

	// Reset mock
	mockPM.updateConfigCalled = false
	mockPM.lastConfig = nil

	// Test case: RequiresFilterUpdate = true
	diffFilter := config.DiffResult{RequiresRestart: false, RequiresFilterUpdate: true}
	HandleConfigChange(mockPM, newCfg, diffFilter)
	if !mockPM.updateConfigCalled || mockPM.lastConfig != newCfg {
		t.Errorf("Expected UpdateConfig to be called with newCfg for RequiresFilterUpdate")
	}

	// Reset mock
	mockPM.updateConfigCalled = false
	mockPM.lastConfig = nil

	// Test case: No changes required
	diffNone := config.DiffResult{RequiresRestart: false, RequiresFilterUpdate: false}
	HandleConfigChange(mockPM, newCfg, diffNone)
	if mockPM.updateConfigCalled {
		t.Errorf("Expected UpdateConfig NOT to be called when no changes required")
	}
}
