package mixer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/engine/source"
	"github.com/user/VLX_VisionBridge/internal/models"
)

const PaddingPercentage = 5

// IsSafeFilterValue validates that a filter option only contains expected characters
// to prevent FFmpeg filter injection attacks.
func IsSafeFilterValue(val string) bool {
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

func handleLayerScaling(layer models.Layer) string {
	scaleFilter := ""
	if layer.Scale != "" && layer.Scale != "100%" && IsSafeFilterValue(layer.Scale) {
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
	if layer.Crop != "" && layer.Crop != "none" && IsSafeFilterValue(layer.Crop) {
		cropFilter = fmt.Sprintf(",crop=%s", layer.Crop)
	}
	return scaleFilter + cropFilter
}

func getOverlayPosition(layer models.Layer, padX, padY int) (string, string) {
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
			if IsSafeFilterValue(x) && IsSafeFilterValue(y) {
				overlayX, overlayY = x, y
			}
		}
	}
	return overlayX, overlayY
}

func BuildFilterComplex(cfg *models.Config, padX, padY int) ([]string, string, string) {
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

		args = append(args, source.BuildInputArgs(layer)...)

		var buf [24]byte
		inputPad := string(append(strconv.AppendInt(append(buf[:0], '['), int64(inputIdx), 10), ":v]"...))
		scaledPad := string(append(strconv.AppendInt(append(buf[:0], "[v"...), int64(i), 10), "_scaled]"...))

		scaleCropFilter := handleLayerScaling(layer)

		filterComplex.WriteString(inputPad)
		filterComplex.WriteString(" ")
		filterComplex.WriteString(scaleCropFilter)
		filterComplex.WriteString(" ")
		filterComplex.WriteString(scaledPad)
		filterComplex.WriteString(";\n")

		overlayX, overlayY := getOverlayPosition(layer, padX, padY)

		outPad := string(append(strconv.AppendInt(append(buf[:0], "[out"...), int64(i), 10), ']'))
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
