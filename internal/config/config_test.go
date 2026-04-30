package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	yamlContent := `
output:
  resolution: "1920x1080"
  fps: 60
  video_bitrate: "6000k"
  audio_bitrate: "160k"
layers:
  - id: 1
    active: true
    input_type: "folder"
    input_path: "/path/to/folder"
    media: "Video+Audio"
    scale: "100%"
    crop: "none"
    position: "center"
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
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
	if len(cfg.Layers) != 1 {
		t.Fatalf("Expected 1 layer, got %d", len(cfg.Layers))
	}
	if cfg.Layers[0].ID != 1 {
		t.Errorf("Expected layer ID 1, got %d", cfg.Layers[0].ID)
	}
}

func TestDiffConfigs(t *testing.T) {
	oldCfg := &Config{
		Output: OutputSettings{Resolution: "1920x1080", FPS: 30},
		Layers: []Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "test.mp4", Scale: "100%"},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Scale: "50%"},
		},
	}

	// Test case 1: No change
	newCfg1 := &Config{
		Output: OutputSettings{Resolution: "1920x1080", FPS: 30},
		Layers: []Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "test.mp4", Scale: "100%"},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Scale: "50%"},
		},
	}
	diff1 := DiffConfigs(oldCfg, newCfg1)
	if diff1.RequiresRestart || diff1.RequiresFilterUpdate {
		t.Errorf("Expected no changes, got requiresRestart=%v, requiresFilterUpdate=%v", diff1.RequiresRestart, diff1.RequiresFilterUpdate)
	}

	// Test case 2: Output change -> requires restart
	newCfg2 := &Config{
		Output: OutputSettings{Resolution: "1280x720", FPS: 30},
		Layers: oldCfg.Layers,
	}
	diff2 := DiffConfigs(oldCfg, newCfg2)
	if !diff2.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for output change")
	}

	// Test case 3: Filter update (change active state)
	newCfg3 := &Config{
		Output: oldCfg.Output,
		Layers: []Layer{
			{ID: 1, Active: false, InputType: "loop", InputPath: "test.mp4", Scale: "100%"},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Scale: "50%"},
		},
	}
	diff3 := DiffConfigs(oldCfg, newCfg3)
	if diff3.RequiresRestart || !diff3.RequiresFilterUpdate {
		t.Errorf("Expected requiresFilterUpdate=true and requiresRestart=false for active state change")
	}

	// Test case 4: Input path change -> requires restart
	newCfg4 := &Config{
		Output: oldCfg.Output,
		Layers: []Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "new.mp4", Scale: "100%"},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Scale: "50%"},
		},
	}
	diff4 := DiffConfigs(oldCfg, newCfg4)
	if !diff4.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for input path change")
	}
}

func TestWatcher(t *testing.T) {
	yamlContent1 := `
output:
  resolution: "1920x1080"
  fps: 60
layers:
  - id: 1
    active: true
    input_type: "folder"
    input_path: "/path1"
`
	yamlContent2 := `
output:
  resolution: "1920x1080"
  fps: 60
layers:
  - id: 1
    active: false
    input_type: "folder"
    input_path: "/path1"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(yamlContent1), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	ch := make(chan DiffResult, 1)
	watcher := NewWatcher(configFile, func(cfg *Config, diff DiffResult) {
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
		if diff.RequiresRestart || !diff.RequiresFilterUpdate {
			t.Errorf("Expected requiresFilterUpdate=true, got %v", diff)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for watcher callback")
	}
}
