package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/go-live-orchestrator/internal/models"
)

// BuildFFmpegArgs generates the FFmpeg arguments based on the provided configuration.
func BuildFFmpegArgs(cfg *models.Config) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	var args []string
	var filterComplex strings.Builder

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

	activeLayerCount := 0

	for _, layer := range cfg.Layers {
		if layer.Active {
			activeLayerCount++
		}
	}

	if activeLayerCount == 0 {
		return args, nil
	}

	// Create base canvas
	filterComplex.WriteString(fmt.Sprintf("color=s=%s:c=black [base];\n", cfg.Output.Resolution))

	inputIdx := 0
	currentBasePad := "[base]"

	for i, layer := range cfg.Layers {
		if !layer.Active {
			continue
		}

		// 1. Handle inputs
		switch layer.InputType {
		case "folder":
			args = append(args, "-f", "image2", "-loop", "1", "-i", layer.InputPath)
		case "loop":
			args = append(args, "-stream_loop", "-1", "-i", layer.InputPath)
		case "srt":
			// Assuming standard srt URL
			args = append(args, "-fflags", "nobuffer", "-flags", "low_delay", "-i", layer.InputPath)
		default:
			// Fallback generic input
			args = append(args, "-i", layer.InputPath)
		}

		// 2. Build filter complex for this layer
		inputPad := fmt.Sprintf("[%d:v]", inputIdx)
		scaledPad := fmt.Sprintf("[v%d_scaled]", i)

		// Scale/Crop logic
		// Example: scale=1920x1080
		scaleFilter := ""
		if layer.Scale != "" && layer.Scale != "100%" {
			if strings.HasSuffix(layer.Scale, "%") {
				// percentage scaling
				pctStr := strings.TrimSuffix(layer.Scale, "%")
				pct, err := strconv.Atoi(pctStr)
				if err == nil {
					scaleFilter = fmt.Sprintf("scale=iw*%d/100:ih*%d/100", pct, pct)
				} else {
					scaleFilter = "scale=iw:ih" // default no-op
				}
			} else {
				// absolute scaling
				scaleFilter = fmt.Sprintf("scale=%s", layer.Scale)
			}
		} else {
			scaleFilter = "copy"
		}

		cropFilter := ""
		if layer.Crop != "" && layer.Crop != "none" {
			cropFilter = fmt.Sprintf(",crop=%s", layer.Crop)
		}

		// Write scale/crop filter
		if scaleFilter == "copy" && cropFilter == "" {
			filterComplex.WriteString(fmt.Sprintf("%s copy %s;\n", inputPad, scaledPad))
		} else {
			filterComplex.WriteString(fmt.Sprintf("%s %s%s %s;\n", inputPad, scaleFilter, cropFilter, scaledPad))
		}

		// Position logic
		overlayX := "0"
		overlayY := "0"

		switch layer.Position {
		case "center":
			overlayX = "(W-w)/2"
			overlayY = "(H-h)/2"
		case "top-left":
			overlayX = fmt.Sprintf("%d", padX)
			overlayY = fmt.Sprintf("%d", padY)
		case "top-right":
			overlayX = fmt.Sprintf("W-w-%d", padX)
			overlayY = fmt.Sprintf("%d", padY)
		case "bottom-left":
			overlayX = fmt.Sprintf("%d", padX)
			overlayY = fmt.Sprintf("H-h-%d", padY)
		case "bottom-right":
			overlayX = fmt.Sprintf("W-w-%d", padX)
			overlayY = fmt.Sprintf("H-h-%d", padY)
		default:
			// parse exact coords like "100:200" or just default to 0:0
			parts := strings.Split(layer.Position, ":")
			if len(parts) == 2 {
				overlayX = parts[0]
				overlayY = parts[1]
			}
		}

		// Overlay onto the current base pad
		outPad := fmt.Sprintf("[out%d]", i)
		filterComplex.WriteString(fmt.Sprintf("%s%s overlay=x=%s:y=%s %s;\n", currentBasePad, scaledPad, overlayX, overlayY, outPad))

		currentBasePad = outPad
		inputIdx++
	}

	args = append(args, "-filter_complex", filterComplex.String())
	args = append(args, "-map", currentBasePad)

	// Add global output settings
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

	// Implement the tee muxer logic to output to multiple destinations
	if len(cfg.Output.Destinations) > 0 {
		var teeDestinations []string
		for _, dest := range cfg.Output.Destinations {
			// Escape '\' and '|' to prevent tee muxer injection
			escaped := strings.ReplaceAll(dest, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "|", "\\|")
			teeDestinations = append(teeDestinations, fmt.Sprintf("[f=flv]%s", escaped))
		}

		teeMap := strings.Join(teeDestinations, "|")
		args = append(args, "-f", "tee", teeMap)
	}

	return args, nil
}
