package engine

import (
	"strings"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestBuildFFmpegArgs_FilterInjection(t *testing.T) {
	cfg := &models.Config{
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
		},
		Layers: []models.Layer{
			{
				ID:        0,
				Active:    true,
				InputType: "loop",
				InputPath: "video.mp4",
				Scale:     "100:100,drawtext=text='pwned'",
				Crop:      "100:100;[v0_scaled]...",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, "drawtext") {
		t.Errorf("Scale filter injection successful: %s", argsStr)
	}
	if strings.Contains(argsStr, "pwned") {
		t.Errorf("Crop filter injection successful: %s", argsStr)
	}
}

func TestBuildFFmpegArgs_InputPathInjection(t *testing.T) {
	cfg := &models.Config{
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
		},
		Layers: []models.Layer{
			{
				ID:        0,
				Active:    true,
				InputType: "default",
				InputPath: "-vcodec",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")

	// -vcodec shouldn't be parsed directly after -i without ./
	if strings.Contains(argsStr, " -i -vcodec ") || strings.HasSuffix(argsStr, " -i -vcodec") {
		t.Errorf("InputPath argument injection successful: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-i ./-vcodec") {
		t.Errorf("InputPath should be sanitized: %s", argsStr)
	}
}

func TestBuildFFmpegArgs_PositionInjection(t *testing.T) {
	cfg := &models.Config{
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
		},
		Layers: []models.Layer{
			{
				ID:        0,
				Active:    true,
				InputType: "loop",
				InputPath: "video.mp4",
				Scale:     "100%",
				Crop:      "none",
				Position:  "0,drawtext=text='pwned':0",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, "drawtext") || strings.Contains(argsStr, "pwned") {
		t.Errorf("Position filter injection successful: %s", argsStr)
	}
	// It should fallback to default 0:0 since position injection was blocked
	if !strings.Contains(argsStr, "overlay=x=0:y=0") {
		t.Errorf("Expected fallback to overlay=x=0:y=0, got: %s", argsStr)
	}
}
