package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/engine/mixer"
	"github.com/user/VLX_VisionBridge/internal/engine/streamer"
	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFFmpegArgs(cfg *models.Config) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Parsing risoluzione
	inputResParts := strings.Split(cfg.Input.Resolution, "x")
	if len(inputResParts) != 2 {
		return nil, fmt.Errorf("invalid input resolution format, expected WxH: %s", cfg.Input.Resolution)
	}
	_, errInputW := strconv.Atoi(inputResParts[0])
	_, errInputH := strconv.Atoi(inputResParts[1])
	if errInputW != nil || errInputH != nil {
		return nil, fmt.Errorf("invalid input resolution values: %s", cfg.Input.Resolution)
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

	// 1. Costruisce la pipeline di mixaggio (Sorgenti -> Encoder -> TEE)
	argsFilter, _, _, _ := mixer.BuildFilterComplex(cfg)
	args = append(args, argsFilter...)

	// 2. Costruisce l'output (TEE -> Muxer -> Sink RTMP/SRT)
	outArgs, err := streamer.BuildOutputArgs(cfg)
	if err != nil {
		return nil, err
	}
	args = append(args, outArgs...)

	return args, nil
}
