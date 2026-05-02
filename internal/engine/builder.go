package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/go-live-orchestrator/internal/models"
)

// isSafeFilterValue validates that a filter option only contains expected characters
// to prevent FFmpeg filter injection attacks.
func isSafeFilterValue(val string) bool {
	for _, c := range val {
		if (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			c == ':' || c == '%' || c == '-' || c == '_' ||
			c == '/' || c == '*' || c == '+' || c == '.' {
			continue
		}
		return false
	}
	return true
}

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
	outW, errW := strconv.Atoi(resParts[0])
	outH, errH := strconv.Atoi(resParts[1])
	if errW != nil || errH != nil {
		return nil, fmt.Errorf("invalid resolution values: %s", cfg.Output.Resolution)
	}

	// Calculate 5% padding
	padX := outW * 5 / 100
	padY := outH * 5 / 100

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
	argsFilter, filterComplex, lastPad := buildFilterComplex(cfg, padX, padY)
	args = append(args, argsFilter...)
	args = append(args, "-filter_complex", filterComplex)
	args = append(args, "-map", lastPad)
	args = append(args, buildOutputArgs(cfg)...)

	return args, nil
}

func handleLayerScaling(layer models.Layer) string {
	scaleFilter := ""
	if layer.Scale != "" && layer.Scale != "100%" && isSafeFilterValue(layer.Scale) {
		if strings.HasSuffix(layer.Scale, "%") {
			pctStr := strings.TrimSuffix(layer.Scale, "%")
			pct, err := strconv.Atoi(pctStr)
			if err == nil {
				scaleFilter = fmt.Sprintf("scale=iw*%d/100:ih*%d/100", pct, pct)
			} else {
				scaleFilter = "scale=iw:ih"
			}
		} else {
			scaleFilter = fmt.Sprintf("scale=%s", layer.Scale)
		}
	} else {
		scaleFilter = "copy"
	}

	cropFilter := ""
	if layer.Crop != "" && layer.Crop != "none" && isSafeFilterValue(layer.Crop) {
		cropFilter = fmt.Sprintf(",crop=%s", layer.Crop)
	}
	return scaleFilter + cropFilter
}

func buildOutputArgs(cfg *models.Config) []string {
	var args []string
	if cfg.Output.Resolution != "" {
		args = append(args, "-s", cfg.Output.Resolution)
	}
	if cfg.Output.FPS > 0 {
		args = append(args, "-r", strconv.Itoa(cfg.Output.FPS))
	}
	if cfg.Output.VideoBitrate != "" {
		args = append(args, "-c:v", "libx264", "-b:v", cfg.Output.VideoBitrate, "-maxrate", cfg.Output.VideoBitrate, "-bufsize", cfg.Output.VideoBitrate)
	}
	if cfg.Output.AudioBitrate != "" {
		args = append(args, "-c:a", "aac", "-b:a", cfg.Output.AudioBitrate)
	}

	if len(cfg.Output.Destinations) > 0 {
		var teeDestinations []string
		for _, dest := range cfg.Output.Destinations {
			escaped := strings.ReplaceAll(dest, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "|", "\\|")
			teeDestinations = append(teeDestinations, fmt.Sprintf("[f=flv]%s", escaped))
		}
		teeMap := strings.Join(teeDestinations, "|")
		args = append(args, "-f", "tee", teeMap)
	}
	return args
}

func buildFilterComplex(cfg *models.Config, padX, padY int) ([]string, string, string) {
	var args []string
	var filterComplex strings.Builder
	filterComplex.WriteString(fmt.Sprintf("color=s=%s:c=black [base];\n", cfg.Output.Resolution))

	inputIdx := 0
	currentBasePad := "[base]"

	for i, layer := range cfg.Layers {
		if !layer.Active {
			continue
		}

		switch layer.InputType {
		case "folder":
			args = append(args, "-f", "image2", "-loop", "1", "-i", layer.InputPath)
		case "loop":
			args = append(args, "-stream_loop", "-1", "-i", layer.InputPath)
		case "srt":
			args = append(args, "-fflags", "nobuffer", "-flags", "low_delay", "-i", layer.InputPath)
		default:
			args = append(args, "-i", layer.InputPath)
		}

		inputPad := fmt.Sprintf("[%d:v]", inputIdx)
		scaledPad := fmt.Sprintf("[v%d_scaled]", i)

		scaleCropFilter := handleLayerScaling(layer)
		if scaleCropFilter == "copy" {
			filterComplex.WriteString(fmt.Sprintf("%s copy %s;\n", inputPad, scaledPad))
		} else {
			filterComplex.WriteString(fmt.Sprintf("%s %s %s;\n", inputPad, scaleCropFilter, scaledPad))
		}

		overlayX, overlayY := "0", "0"
		switch layer.Position {
		case "center":
			overlayX, overlayY = "(W-w)/2", "(H-h)/2"
		case "top-left":
			overlayX, overlayY = fmt.Sprintf("%d", padX), fmt.Sprintf("%d", padY)
		case "top-right":
			overlayX, overlayY = fmt.Sprintf("W-w-%d", padX), fmt.Sprintf("%d", padY)
		case "bottom-left":
			overlayX, overlayY = fmt.Sprintf("%d", padX), fmt.Sprintf("H-h-%d", padY)
		case "bottom-right":
			overlayX, overlayY = fmt.Sprintf("W-w-%d", padX), fmt.Sprintf("H-h-%d", padY)
		default:
			if x, y, found := strings.Cut(layer.Position, ":"); found && !strings.Contains(y, ":") {
				overlayX, overlayY = x, y
			}
		}
		outPad := fmt.Sprintf("[out%d]", i)
		filterComplex.WriteString(fmt.Sprintf("%s%s overlay=x=%s:y=%s %s;\n", currentBasePad, scaledPad, overlayX, overlayY, outPad))
		currentBasePad = outPad
		inputIdx++
	}
	return args, filterComplex.String(), currentBasePad
}
