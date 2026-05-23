package mixer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/engine/source"
	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string
	var filterComplex strings.Builder
	filterComplex.WriteString("color=s=")
	filterComplex.WriteString(cfg.Input.Resolution)
	filterComplex.WriteString(":c=black [base];\n")

	inputIdx := 0
	currentBasePad := "[base]"
	var audioPads []string

	for i, layer := range cfg.Layers {
		if !layer.Active {
			continue
		}

		res := source.BuildInputArgs(layer)
		args = append(args, res.Args...)

		media := layer.Media
		if media == "" {
			media = "Video+Audio"
		}

		layerVideoPad := ""
		layerAudioPad := ""

		if res.InputCount == 1 {
			layerVideoPad = string(append(strconv.AppendInt([]byte("["), int64(inputIdx), 10), ":v]"...))
			layerAudioPad = string(append(strconv.AppendInt([]byte("["), int64(inputIdx), 10), ":a]"...))
		} else if res.InputCount == 2 {
			layerVideoPad = string(append(strconv.AppendInt([]byte("["), int64(inputIdx), 10), ":v]"...))
			layerAudioPad = string(append(strconv.AppendInt([]byte("["), int64(inputIdx+1), 10), ":a]"...))
		}

		// Handle Video
		if (media == "Video" || media == "Video+Audio") && res.HasVideo {
			var buf [24]byte
			scaledPad := string(append(strconv.AppendInt(append(buf[:0], "[v"...), int64(i), 10), "_scaled]"...))
			outPad := string(append(strconv.AppendInt(append(buf[:0], "[out"...), int64(i), 10), ']'))

			filterComplex.WriteString(layerVideoPad)
			filterComplex.WriteString(" scale=")
			filterComplex.WriteString(strconv.Itoa(layer.Size))
			filterComplex.WriteString(":-1 ")
			filterComplex.WriteString(scaledPad)
			filterComplex.WriteString(";\n")

			filterComplex.WriteString(currentBasePad)
			filterComplex.WriteString(scaledPad)
			filterComplex.WriteString(" overlay=x=")
			filterComplex.WriteString(strconv.Itoa(layer.X))
			filterComplex.WriteString(":y=")
			filterComplex.WriteString(strconv.Itoa(layer.Y))
			filterComplex.WriteString(" ")
			filterComplex.WriteString(outPad)
			filterComplex.WriteString(";\n")
			currentBasePad = outPad
		}

		// Handle Audio
		if (media == "Audio" || media == "Video+Audio") && res.HasAudio {
			var buf [24]byte
			aOutPad := string(append(strconv.AppendInt(append(buf[:0], "[a"...), int64(i), 10), ']'))

			volumeFilter := "anull"
			if layer.Volume != nil {
				volumeFilter = fmt.Sprintf("volume=%.2f", float64(*layer.Volume)/100.0)
			}

			filterComplex.WriteString(layerAudioPad)
			filterComplex.WriteString(" ")
			filterComplex.WriteString(volumeFilter)
			filterComplex.WriteString(" ")
			filterComplex.WriteString(aOutPad)
			filterComplex.WriteString(";\n")

			audioPads = append(audioPads, aOutPad)
		}

		inputIdx += res.InputCount
	}

	var finalAudioPad string
	if len(audioPads) == 0 {
		filterComplex.WriteString("anullsrc=r=44100:cl=stereo [a_out];\n")
		finalAudioPad = "[a_out]"
	} else if len(audioPads) == 1 {
		finalAudioPad = audioPads[0]
	} else {
		for _, pad := range audioPads {
			filterComplex.WriteString(pad)
		}
		filterComplex.WriteString(" amix=inputs=")
		filterComplex.WriteString(strconv.Itoa(len(audioPads)))
		filterComplex.WriteString(":duration=longest [a_out];\n")
		finalAudioPad = "[a_out]"
	}

	return args, filterComplex.String(), currentBasePad, finalAudioPad
}
