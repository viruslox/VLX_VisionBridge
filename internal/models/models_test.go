package models

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfig_UnmarshalYAML(t *testing.T) {
	yamlData := []byte(`
database:
  dsn: "/opt/VLX_VisionBridge/var/visionbridge.db"
output:
  resolution: "1920x1080"
  fps: 60
  video_bitrate: "6000k"
  audio_bitrate: "160k"
  destinations:
    - "rtmp://live.twitch.tv/app/live_xyz"
    - "rtmp://a.rtmp.youtube.com/live2/xyz"
layers:
  - id: 1
    active: true
    input_type: "srt"
    input_path: "srt://localhost:9000?mode=listener"
    media: "Video+Audio"
    size: 1920
    x: 0
    y: 0
  - id: 2
    active: false
    input_type: "folder"
    input_path: "/var/media/loop"
    media: "Video Only"
    size: 640
    x: 10
    y: 10
`)

	var cfg Config
	err := yaml.Unmarshal(yamlData, &cfg)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify Database Config
	if cfg.Database.DSN != "/opt/VLX_VisionBridge/var/visionbridge.db" {
		t.Errorf("Expected DSN to be '/opt/VLX_VisionBridge/var/visionbridge.db', got '%s'", cfg.Database.DSN)
	}

	// Verify Output Settings
	if cfg.Output.Resolution != "1920x1080" {
		t.Errorf("Expected Output.Resolution to be '1920x1080', got '%s'", cfg.Output.Resolution)
	}
	if cfg.Output.FPS != 60 {
		t.Errorf("Expected Output.FPS to be 60, got %d", cfg.Output.FPS)
	}
	if cfg.Output.VideoBitrate != "6000k" {
		t.Errorf("Expected Output.VideoBitrate to be '6000k', got '%s'", cfg.Output.VideoBitrate)
	}
	if cfg.Output.AudioBitrate != "160k" {
		t.Errorf("Expected Output.AudioBitrate to be '160k', got '%s'", cfg.Output.AudioBitrate)
	}
	if len(cfg.Output.Destinations) != 2 {
		t.Fatalf("Expected 2 destinations, got %d", len(cfg.Output.Destinations))
	}
	if cfg.Output.Destinations[0] != "rtmp://live.twitch.tv/app/live_xyz" {
		t.Errorf("Expected first destination to be 'rtmp://live.twitch.tv/app/live_xyz', got '%s'", cfg.Output.Destinations[0])
	}
	if cfg.Output.Destinations[1] != "rtmp://a.rtmp.youtube.com/live2/xyz" {
		t.Errorf("Expected second destination to be 'rtmp://a.rtmp.youtube.com/live2/xyz', got '%s'", cfg.Output.Destinations[1])
	}

	// Verify Layers
	if len(cfg.Layers) != 2 {
		t.Fatalf("Expected 2 layers, got %d", len(cfg.Layers))
	}

	// Layer 1
	layer1 := cfg.Layers[0]
	if layer1.ID != 1 {
		t.Errorf("Expected Layer 1 ID to be 1, got %d", layer1.ID)
	}
	if !layer1.Active {
		t.Errorf("Expected Layer 1 Active to be true")
	}
	if layer1.InputType != "srt" {
		t.Errorf("Expected Layer 1 InputType to be 'srt', got '%s'", layer1.InputType)
	}
	if layer1.InputPath != "srt://localhost:9000?mode=listener" {
		t.Errorf("Expected Layer 1 InputPath to be 'srt://localhost:9000?mode=listener', got '%s'", layer1.InputPath)
	}
	if layer1.Media != "Video+Audio" {
		t.Errorf("Expected Layer 1 Media to be 'Video+Audio', got '%s'", layer1.Media)
	}
	if layer1.Size != 1920 {
		t.Errorf("Expected Layer 1 Size to be 1920, got %d", layer1.Size)
	}
	if layer1.X != 0 {
		t.Errorf("Expected Layer 1 X to be 0, got %d", layer1.X)
	}
	if layer1.Y != 0 {
		t.Errorf("Expected Layer 1 Y to be 0, got %d", layer1.Y)
	}

	// Layer 2
	layer2 := cfg.Layers[1]
	if layer2.ID != 2 {
		t.Errorf("Expected Layer 2 ID to be 2, got %d", layer2.ID)
	}
	if layer2.Active {
		t.Errorf("Expected Layer 2 Active to be false")
	}
	if layer2.InputType != "folder" {
		t.Errorf("Expected Layer 2 InputType to be 'folder', got '%s'", layer2.InputType)
	}
	if layer2.InputPath != "/var/media/loop" {
		t.Errorf("Expected Layer 2 InputPath to be '/var/media/loop', got '%s'", layer2.InputPath)
	}
	if layer2.Media != "Video Only" {
		t.Errorf("Expected Layer 2 Media to be 'Video Only', got '%s'", layer2.Media)
	}
	if layer2.Size != 640 {
		t.Errorf("Expected Layer 2 Size to be 640, got %d", layer2.Size)
	}
	if layer2.X != 10 {
		t.Errorf("Expected Layer 2 X to be 10, got %d", layer2.X)
	}
	if layer2.Y != 10 {
		t.Errorf("Expected Layer 2 Y to be 10, got %d", layer2.Y)
	}
}
