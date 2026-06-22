package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestLoadConfig(t *testing.T) {
	yamlContent := `
output:
  resolution: "1920x1080"
  fps: 60
  video_bitrate: "6000k"
  audio_bitrate: "160k"
input:
  resolution: "1920x1080"
  media_source:
    layers:
      - id: 1
        active: true
        input_type: "folder"
        input_path: "/path/to/folder"
        media: "Video+Audio"
        size: 1920
        x: 0
        y: 0
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "visionbridge.settings")
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Output.Resolution != "1920x1080" {
		t.Errorf("Expected resolution '1920x1080', got '%s'", cfg.Output.Resolution)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/path/to/non/existent/file.yaml")
	if err == nil {
		t.Errorf("Expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	invalidYaml := `
output:
  resolution: "1920x1080"
	invalid_indentation
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid_visionbridge.settings")
	err := os.WriteFile(configFile, []byte(invalidYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Errorf("Expected error for invalid YAML, got nil")
	}
}

func TestDiffConfigs(t *testing.T) {
	oldCfg := &models.Config{
		Input: models.InputSettings{
			Resolution: "1920x1080",
			ChromiumSource: models.ChromiumSource{
				Active: true,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080", FPS: 30},
	}

	// Test case 1: No change
	newCfg1 := &models.Config{
		Input: models.InputSettings{
			Resolution: "1920x1080",
			ChromiumSource: models.ChromiumSource{
				Active: true,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080", FPS: 30},
	}
	diff1 := DiffConfigs(oldCfg, newCfg1)
	if diff1.RequiresRestart || diff1.RequiresFilterUpdate {
		t.Errorf("Expected no changes, got requiresRestart=%v, requiresFilterUpdate=%v", diff1.RequiresRestart, diff1.RequiresFilterUpdate)
	}

	// Test case 2: Output change -> requires restart
	newCfg2 := &models.Config{
		Input:  oldCfg.Input,
		Output: models.OutputSettings{Resolution: "1280x720", FPS: 30},
	}
	diff2 := DiffConfigs(oldCfg, newCfg2)
	if !diff2.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for output change")
	}
}

func TestWatcher(t *testing.T) {
	yamlContent1 := `
output:
  resolution: "1920x1080"
  fps: 60
input:
  chromium_source:
    active: true
`
	yamlContent2 := `
output:
  resolution: "1920x1080"
  fps: 60
input:
  chromium_source:
    active: false
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "visionbridge.settings")
	err := os.WriteFile(configFile, []byte(yamlContent1), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	ch := make(chan DiffResult, 1)
	watcher := NewWatcher(configFile, func(cfg *models.Config, diff DiffResult) {
		ch <- diff
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Wait a bit for the watcher to initialize
	time.Sleep(100 * time.Millisecond)

	// Update the file
	err = os.WriteFile(configFile, []byte(yamlContent2), 0644)
	if err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	select {
	case diff := <-ch:
		if !diff.RequiresRestart || diff.RequiresFilterUpdate {
			t.Errorf("Expected requiresRestart=true for active state change, got %v", diff)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for watcher callback")
	}
}
