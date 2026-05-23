package engine

import (
	"strings"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

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
