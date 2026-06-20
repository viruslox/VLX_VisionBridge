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

	// Parse input resolution
	inputResParts := strings.Split(cfg.Input.Resolution, "x")
	if len(inputResParts) != 2 {
		return nil, fmt.Errorf("invalid input resolution format, expected WxH: %s", cfg.Input.Resolution)
	}
	_, errInputW := strconv.Atoi(inputResParts[0])
	_, errInputH := strconv.Atoi(inputResParts[1])
	if errInputW != nil || errInputH != nil {
		return nil, fmt.Errorf("invalid input resolution values: %s", cfg.Input.Resolution)
	}

	// Parse output resolution
	resParts := strings.Split(cfg.Output.Resolution, "x")
	if len(resParts) != 2 {
		return nil, fmt.Errorf("invalid output resolution format, expected WxH: %s", cfg.Output.Resolution)
	}
	_, errW := strconv.Atoi(resParts[0])
	_, errH := strconv.Atoi(resParts[1])
	if errW != nil || errH != nil {
		return nil, fmt.Errorf("invalid output resolution values: %s", cfg.Output.Resolution)
	}

	hasActiveLayer := false
	if cfg.Input.MediaSource.Active {
		for _, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				hasActiveLayer = true
				break
			}
		}
	}
	if cfg.Input.ChromiumSource.Active {
		hasActiveLayer = true
	}

	if !hasActiveLayer {
		return []string{}, nil
	}

	var args []string
	args = append(args, "-fflags", "nobuffer+genpts", "-flags", "low_delay")

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
