package engine

import (
	"strings"
	"testing"

	"github.com/user/go-live-orchestrator/internal/models"
)

func TestBuildFFmpegArgs(t *testing.T) {
	cfg := &models.Config{
		Output: models.OutputSettings{
			Resolution:   "1920x1080",
			FPS:          60,
			VideoBitrate: "6000k",
			AudioBitrate: "160k",
			Destinations: []string{
				"rtmp://live.twitch.tv/app/live_xyz",
				"rtmp://a.rtmp.youtube.com/live2/xyz",
			},
		},
		Layers: []models.Layer{
			{
				ID:        0,
				Active:    true,
				InputType: "folder",
				InputPath: "/images/",
				Scale:     "100%",
				Position:  "top-left",
			},
			{
				ID:        1,
				Active:    true,
				InputType: "loop",
				InputPath: "video.mp4",
				Scale:     "50%",
				Position:  "center",
			},
			{
				ID:        2,
				Active:    false, // Should be ignored
				InputType: "srt",
				InputPath: "srt://example.com:1234",
				Position:  "top-left",
			},
			{
				ID:        3,
				Active:    true,
				InputType: "srt",
				InputPath: "srt://example.com:5678",
				Scale:     "1280x720",
				Position:  "10:20",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")

	// 1. Verify inputs
	if !strings.Contains(argsStr, "-f image2 -loop 1 -i /images/") {
		t.Errorf("Missing folder input: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-stream_loop -1 -i video.mp4") {
		t.Errorf("Missing loop input: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-i srt://example.com:5678") {
		t.Errorf("Missing active srt input: %s", argsStr)
	}
	if strings.Contains(argsStr, "srt://example.com:1234") {
		t.Errorf("Included inactive srt input: %s", argsStr)
	}

	// 2. Verify filter complex
	var filterComplexStr string
	for i, arg := range args {
		if arg == "-filter_complex" && i+1 < len(args) {
			filterComplexStr = args[i+1]
			break
		}
	}
	if filterComplexStr == "" {
		t.Fatalf("Missing -filter_complex flag")
	}

	// 5% padding of 1920x1080 = 96x54
	// Layer 0 (top-left) -> padding applied
	if !strings.Contains(filterComplexStr, "overlay=x=96:y=54 [out0]") {
		t.Errorf("Layer 0 missing top-left padding overlay: %s", filterComplexStr)
	}

	// Layer 1 (center)
	if !strings.Contains(filterComplexStr, "overlay=x=(W-w)/2:y=(H-h)/2 [out1]") {
		t.Errorf("Layer 1 missing center overlay: %s", filterComplexStr)
	}

	// Layer 3 (custom pos 10:20)
	// Notice index is 3 in layers array, so input is [3:v] and out is [out3]
	if !strings.Contains(filterComplexStr, "overlay=x=10:y=20 [out3]") {
		t.Errorf("Layer 3 missing custom pos overlay: %s", filterComplexStr)
	}

	// Verify scaling logic
	// Layer 1 scale 50%
	if !strings.Contains(filterComplexStr, "scale=iw*50/100:ih*50/100 [v1_scaled]") {
		t.Errorf("Layer 1 missing 50%% scale: %s", filterComplexStr)
	}

	// Layer 3 absolute scale
	if !strings.Contains(filterComplexStr, "scale=1280x720 [v3_scaled]") {
		t.Errorf("Layer 3 missing absolute scale: %s", filterComplexStr)
	}

	// 3. Verify final map
	if !strings.Contains(argsStr, "-map [out3]") {
		t.Errorf("Missing final map to last active layer: %s", argsStr)
	}

	// 4. Verify global settings
	if !strings.Contains(argsStr, "-s 1920x1080") {
		t.Errorf("Missing Resolution setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-r 60") {
		t.Errorf("Missing FPS setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-c:v libx264 -b:v 6000k -maxrate 6000k -bufsize 6000k") {
		t.Errorf("Missing VideoBitrate setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-c:a aac -b:a 160k") {
		t.Errorf("Missing AudioBitrate setting: %s", argsStr)
	}

	// 5. Verify tee muxer
	expectedTee := "-f tee [f=flv]rtmp://live.twitch.tv/app/live_xyz|[f=flv]rtmp://a.rtmp.youtube.com/live2/xyz"
	if !strings.Contains(argsStr, expectedTee) {
		t.Errorf("Missing or incorrect tee muxer setting: expected %s in %s", expectedTee, argsStr)
	}
}
