package engine

import (
	"fmt"

	"github.com/user/VLX_VisionBridge/internal/engine/mixer"
	"github.com/user/VLX_VisionBridge/internal/engine/streamer"
	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildPipelineArgs(cfg *models.Config) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	hasActiveLayer := cfg.Input.ChromiumSource.Active
	if cfg.Input.MediaSource.Active {
		for _, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				hasActiveLayer = true
				break
			}
		}
	}

	if !hasActiveLayer {
		return []string{}, nil
	}

	var gstArgs []string

	// 1. Build mixing pipeline (WebRTC + Media -> GStreamer TEEs)
	argsFilter, _, _, _ := mixer.BuildFilterComplex(cfg)
	gstArgs = append(gstArgs, argsFilter...)

	// 2. Build output pipeline (TEEs -> Muxer -> Local MediaMTX Sink)
	outArgs, err := streamer.BuildOutputArgs(cfg)
	if err != nil {
		return nil, err
	}
	gstArgs = append(gstArgs, outArgs...)

	return gstArgs, nil
}
