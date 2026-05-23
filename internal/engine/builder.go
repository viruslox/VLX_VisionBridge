package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/engine/mixer"
	"github.com/user/VLX_VisionBridge/internal/engine/streamer"
	"github.com/user/VLX_VisionBridge/internal/models"
)

// BuildFFmpegArgs generates the FFmpeg arguments based on the provided configuration.
func BuildFFmpegArgs(cfg *models.Config) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Parse output resolution
	resParts := strings.Split(cfg.Output.Resolution, "x")
	if len(resParts) != 2 {
		return nil, fmt.Errorf("invalid resolution format, expected WxH: %s", cfg.Output.Resolution)
	}
	_, errW := strconv.Atoi(resParts[0])
	_, errH := strconv.Atoi(resParts[1])
	if errW != nil || errH != nil {
		return nil, fmt.Errorf("invalid resolution values: %s", cfg.Output.Resolution)
	}

	if len(cfg.Layers) == 0 {
		return []string{}, nil
	}

	hasActiveLayer := false
	for _, layer := range cfg.Layers {
		if layer.Active {
			hasActiveLayer = true
			break
		}
	}

	if !hasActiveLayer {
		return []string{}, nil
	}

	var args []string
	argsFilter, filterComplex, lastVideoPad, finalAudioPad := mixer.BuildFilterComplex(cfg)
	args = append(args, argsFilter...)
	args = append(args, "-filter_complex", filterComplex)
	args = append(args, "-map", lastVideoPad)
	args = append(args, "-map", finalAudioPad)

	outArgs, err := streamer.BuildOutputArgs(cfg)
	if err != nil {
		return nil, err
	}
	args = append(args, outArgs...)

	return args, nil
}
