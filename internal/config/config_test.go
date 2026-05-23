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
	if len(cfg.Layers) != 1 {
		t.Fatalf("Expected 1 layer, got %d", len(cfg.Layers))
	}
	if cfg.Layers[0].ID != 1 {
		t.Errorf("Expected layer ID 1, got %d", cfg.Layers[0].ID)
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
		Input:  models.InputSettings{Resolution: "1920x1080"},
		Output: models.OutputSettings{Resolution: "1920x1080", FPS: 30},
		Layers: []models.Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "test.mp4", Size: 1920},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Size: 960},
		},
	}

	// Test case 1: No change
	newCfg1 := &models.Config{
		Input:  models.InputSettings{Resolution: "1920x1080"},
		Output: models.OutputSettings{Resolution: "1920x1080", FPS: 30},
		Layers: []models.Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "test.mp4", Size: 1920},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Size: 960},
		},
	}
	diff1 := DiffConfigs(oldCfg, newCfg1)
	if diff1.RequiresRestart || diff1.RequiresFilterUpdate {
		t.Errorf("Expected no changes, got requiresRestart=%v, requiresFilterUpdate=%v", diff1.RequiresRestart, diff1.RequiresFilterUpdate)
	}

	// Test case 2: Output change -> requires restart
	newCfg2 := &models.Config{
		Input:  models.InputSettings{Resolution: "1920x1080"},
		Output: models.OutputSettings{Resolution: "1280x720", FPS: 30},
		Layers: oldCfg.Layers,
	}
	diff2 := DiffConfigs(oldCfg, newCfg2)
	if !diff2.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for output change")
	}

	// Test case 3: Filter update (change active state)
	newCfg3 := &models.Config{
		Input:  oldCfg.Input,
		Output: oldCfg.Output,
		Layers: []models.Layer{
			{ID: 1, Active: false, InputType: "loop", InputPath: "test.mp4", Size: 1920},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Size: 960},
		},
	}
	diff3 := DiffConfigs(oldCfg, newCfg3)
	if diff3.RequiresRestart || !diff3.RequiresFilterUpdate {
		t.Errorf("Expected requiresFilterUpdate=true and requiresRestart=false for active state change")
	}

	// Test case 4: Input path change -> requires restart
	newCfg4 := &models.Config{
		Input:  oldCfg.Input,
		Output: oldCfg.Output,
		Layers: []models.Layer{
			{ID: 1, Active: true, InputType: "loop", InputPath: "new.mp4", Size: 1920},
			{ID: 2, Active: false, InputType: "srt", InputPath: "srt://...", Size: 960},
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
		if diff.RequiresRestart || !diff.RequiresFilterUpdate {
			t.Errorf("Expected requiresFilterUpdate=true, got %v", diff)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for watcher callback")
	}
}

func BenchmarkLayersDiff(b *testing.B) {
	oldLayers := []models.Layer{
		{ID: 1, Active: true, InputType: "loop", InputPath: "test1.mp4", Size: 1920},
		{ID: 2, Active: false, InputType: "srt", InputPath: "srt://1", Size: 960},
		{ID: 3, Active: true, InputType: "folder", InputPath: "fld1", Size: 1920},
		{ID: 4, Active: true, InputType: "loop", InputPath: "test2.mp4", Size: 1920},
	}
	newLayers := []models.Layer{
		{ID: 1, Active: true, InputType: "loop", InputPath: "test1.mp4", Size: 1920},
		{ID: 2, Active: true, InputType: "srt", InputPath: "srt://1", Size: 960},
		{ID: 3, Active: true, InputType: "folder", InputPath: "fld1", Size: 1920},
		{ID: 5, Active: true, InputType: "loop", InputPath: "test3.mp4", Size: 1920},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		layersDiff(oldLayers, newLayers)
	}
}

func TestLayersDiff(t *testing.T) {
	tests := []struct {
		name                     string
		old                      []models.Layer
		new                      []models.Layer
		wantRequiresRestart      bool
		wantRequiresFilterUpdate bool
	}{
		{
			name:                     "No changes",
			old:                      []models.Layer{{ID: 1, InputType: "loop", InputPath: "test.mp4", Media: "video", Active: true, Size: 1920, X: 0, Y: 0}},
			new:                      []models.Layer{{ID: 1, InputType: "loop", InputPath: "test.mp4", Media: "video", Active: true, Size: 1920, X: 0, Y: 0}},
			wantRequiresRestart:      false,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "Layer added",
			old:                      []models.Layer{},
			new:                      []models.Layer{{ID: 1}},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "Layer removed",
			old:                      []models.Layer{{ID: 1}},
			new:                      []models.Layer{},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "InputType changed",
			old:                      []models.Layer{{ID: 1, InputType: "loop"}},
			new:                      []models.Layer{{ID: 1, InputType: "srt"}},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "InputPath changed",
			old:                      []models.Layer{{ID: 1, InputPath: "test1.mp4"}},
			new:                      []models.Layer{{ID: 1, InputPath: "test2.mp4"}},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "Media changed",
			old:                      []models.Layer{{ID: 1, Media: "video"}},
			new:                      []models.Layer{{ID: 1, Media: "audio"}},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false,
		},
		{
			name:                     "Active changed",
			old:                      []models.Layer{{ID: 1, Active: true}},
			new:                      []models.Layer{{ID: 1, Active: false}},
			wantRequiresRestart:      false,
			wantRequiresFilterUpdate: true,
		},
		{
			name:                     "Size changed",
			old:                      []models.Layer{{ID: 1, Size: 1920}},
			new:                      []models.Layer{{ID: 1, Size: 960}},
			wantRequiresRestart:      false,
			wantRequiresFilterUpdate: true,
		},
		{
			name:                     "X changed",
			old:                      []models.Layer{{ID: 1, X: 0}},
			new:                      []models.Layer{{ID: 1, X: 100}},
			wantRequiresRestart:      false,
			wantRequiresFilterUpdate: true,
		},
		{
			name:                     "Y changed",
			old:                      []models.Layer{{ID: 1, Y: 0}},
			new:                      []models.Layer{{ID: 1, Y: 100}},
			wantRequiresRestart:      false,
			wantRequiresFilterUpdate: true,
		},
		{
			name:                     "Multiple changes including restart and filter update",
			old:                      []models.Layer{{ID: 1, InputType: "loop", Active: true}},
			new:                      []models.Layer{{ID: 1, InputType: "srt", Active: false}},
			wantRequiresRestart:      true,
			wantRequiresFilterUpdate: false, // Restart takes precedence in effect, but logic sets RequiresRestart=true and may leave RequiresFilterUpdate=false depending on if-else.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layersDiff(tt.old, tt.new)
			if got.RequiresRestart != tt.wantRequiresRestart {
				t.Errorf("RequiresRestart = %v, want %v", got.RequiresRestart, tt.wantRequiresRestart)
			}
			if got.RequiresFilterUpdate != tt.wantRequiresFilterUpdate {
				t.Errorf("RequiresFilterUpdate = %v, want %v", got.RequiresFilterUpdate, tt.wantRequiresFilterUpdate)
			}
		})
	}
}
