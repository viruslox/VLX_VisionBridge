package engine

import (
	"strings"
	"testing"

	"github.com/user/go-live-orchestrator/internal/models"
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
