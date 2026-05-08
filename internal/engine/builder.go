package engine

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/user/go-live-orchestrator/internal/models"
)

var destReplacer = strings.NewReplacer("\\", "\\\\", "|", "\\|")

// isValidDestination strictly validates the destination URL to prevent FFmpeg tee muxer injection.
func isValidDestination(dest string) bool {
	u, err := url.ParseRequestURI(dest)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "rtmp" && scheme != "rtmps" && scheme != "srt" {
		return false
	}

	if u.Host == "" {
		return false
	}

	if strings.ContainsAny(dest, "|\\\"'[]") {
		return false
	}

	return true
}

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

// sanitizeInputPath prevents FFmpeg argument injection by prefixing local paths
// that start with a dash ("-") with "./".
func sanitizeInputPath(path string) string {
	if path == "" {
		return path
	}
	if !strings.Contains(path, "://") && strings.HasPrefix(path, "-") {
		return "./" + path
	}
	return path
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

	outArgs, err := buildOutputArgs(cfg)
	if err != nil {
		return nil, err
	}
	args = append(args, outArgs...)

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


func buildOutputArgs(cfg *models.Config) ([]string, error) {
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
			if !isValidDestination(dest) {
				return nil, fmt.Errorf("invalid or unsafe output destination: %s", dest)
			}
			escaped := destReplacer.Replace(dest)
			teeDestinations = append(teeDestinations, fmt.Sprintf("[f=flv]%s", escaped))
		}
		teeMap := strings.Join(teeDestinations, "|")
		args = append(args, "-f", "tee", teeMap)
	}
	return args, nil
}

func buildFilterComplex(cfg *models.Config, padX, padY int) ([]string, string, string) {
	var args []string
	var filterComplex strings.Builder
	filterComplex.WriteString("color=s=")
	filterComplex.WriteString(cfg.Output.Resolution)
	filterComplex.WriteString(":c=black [base];\n")

	inputIdx := 0
	currentBasePad := "[base]"

	for i, layer := range cfg.Layers {
		if !layer.Active {
			continue
		}

		safePath := sanitizeInputPath(layer.InputPath)
		switch layer.InputType {
		case "folder":
			args = append(args, "-f", "image2", "-loop", "1", "-i", safePath)
		case "loop":
			args = append(args, "-stream_loop", "-1", "-i", safePath)
		case "srt":
			args = append(args, "-fflags", "nobuffer", "-flags", "low_delay", "-i", safePath)
		default:
			args = append(args, "-i", safePath)
		}

		inputPad := "[" + strconv.Itoa(inputIdx) + ":v]"
		scaledPad := "[v" + strconv.Itoa(i) + "_scaled]"

		scaleCropFilter := handleLayerScaling(layer)

		filterComplex.WriteString(inputPad)
		filterComplex.WriteString(" ")
		filterComplex.WriteString(scaleCropFilter)
		filterComplex.WriteString(" ")
		filterComplex.WriteString(scaledPad)
		filterComplex.WriteString(";\n")

		overlayX, overlayY := "0", "0"
		switch layer.Position {
		case "center":
			overlayX, overlayY = "(W-w)/2", "(H-h)/2"
		case "top-left":
			overlayX, overlayY = strconv.Itoa(padX), strconv.Itoa(padY)
		case "top-right":
			overlayX, overlayY = "W-w-" + strconv.Itoa(padX), strconv.Itoa(padY)
		case "bottom-left":
			overlayX, overlayY = strconv.Itoa(padX), "H-h-" + strconv.Itoa(padY)
		case "bottom-right":
			overlayX, overlayY = "W-w-" + strconv.Itoa(padX), "H-h-" + strconv.Itoa(padY)
		default:
			if x, y, found := strings.Cut(layer.Position, ":"); found && !strings.Contains(y, ":") {
				overlayX, overlayY = x, y
			}
		}
		outPad := "[out" + strconv.Itoa(i) + "]"
		filterComplex.WriteString(currentBasePad)
		filterComplex.WriteString(scaledPad)
		filterComplex.WriteString(" overlay=x=")
		filterComplex.WriteString(overlayX)
		filterComplex.WriteString(":y=")
		filterComplex.WriteString(overlayY)
		filterComplex.WriteString(" ")
		filterComplex.WriteString(outPad)
		filterComplex.WriteString(";\n")
		currentBasePad = outPad
		inputIdx++
	}
	return args, filterComplex.String(), currentBasePad
}
