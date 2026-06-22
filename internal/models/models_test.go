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
input:
  resolution: "1920x1080"
  chromium_source:
    active: true
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

	// Verify Input Settings
	if cfg.Input.Resolution != "1920x1080" {
		t.Errorf("Expected Input.Resolution to be '1920x1080', got '%s'", cfg.Input.Resolution)
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

	if !cfg.Input.ChromiumSource.Active {
		t.Errorf("Expected ChromiumSource.Active to be true")
	}
}
