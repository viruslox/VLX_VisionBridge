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

	if cfg.Input.FFmpegSource.Active {
		for i, layer := range cfg.Input.FFmpegSource.Layers {
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
				filterComplex.WriteString(" overlay@layer")
				filterComplex.WriteString(strconv.Itoa(layer.ID))
				filterComplex.WriteString("=x=")
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

				volumeVal := 1.0
				if layer.Volume != nil {
					volumeVal = float64(*layer.Volume) / 100.0
				}

				filterComplex.WriteString(layerAudioPad)
				filterComplex.WriteString(" aresample=async=1,aformat=sample_rates=48000:channel_layouts=stereo,asetpts=N,")
				filterComplex.WriteString(" volume@layer")
				filterComplex.WriteString(strconv.Itoa(layer.ID))
				filterComplex.WriteString("=")
				filterComplex.WriteString(fmt.Sprintf("%.2f", volumeVal))
				filterComplex.WriteString(" ")
				filterComplex.WriteString(aOutPad)
				filterComplex.WriteString(";\n")

				audioPads = append(audioPads, aOutPad)
			}

			inputIdx += res.InputCount
		}
	}

	if cfg.Input.ChromiumSource.Active {
		hasAudio := false
		cs := cfg.Input.ChromiumSource
		if (cs.Z1Volume != nil && *cs.Z1Volume > 0) ||
			(cs.Z2Volume != nil && *cs.Z2Volume > 0) ||
			(cs.Z3Volume != nil && *cs.Z3Volume > 0) ||
			(cs.Z4Volume != nil && *cs.Z4Volume > 0) ||
			(cs.Z5Volume != nil && *cs.Z5Volume > 0) ||
			(cs.Z6Volume != nil && *cs.Z6Volume > 0) ||
			(cs.Z7Volume != nil && *cs.Z7Volume > 0) ||
			(cs.Z8Volume != nil && *cs.Z8Volume > 0) {
			hasAudio = true
		}

		if hasAudio {
			args = append(args, "-f", "x11grab", "-video_size", cfg.Input.Resolution, "-draw_mouse", "0", "-i", ":99", "-f", "pulse", "-i", "default")
		} else {
			args = append(args, "-f", "x11grab", "-video_size", cfg.Input.Resolution, "-draw_mouse", "0", "-i", ":99")
		}

		chromaColor := cfg.Input.ChromiumSource.Z1BgColor
		if chromaColor == "" {
			chromaColor = "#00FF00"
		}
		if strings.HasPrefix(chromaColor, "#") {
			chromaColor = "0x" + chromaColor[1:]
		}

		layerVideoPad := string(append(strconv.AppendInt([]byte("["), int64(inputIdx), 10), ":v]"...))
		chromaPad := "[chroma_chromium]"
		outPad := "[out_chromium]"

		filterComplex.WriteString(layerVideoPad)
		filterComplex.WriteString(" colorkey=")
		filterComplex.WriteString(chromaColor)
		filterComplex.WriteString(":0.1:0.1 ")
		filterComplex.WriteString(chromaPad)
		filterComplex.WriteString(";\n")

		filterComplex.WriteString(currentBasePad)
		filterComplex.WriteString(chromaPad)
		filterComplex.WriteString(" overlay=x=0:y=0 ")
		filterComplex.WriteString(outPad)
		filterComplex.WriteString(";\n")
		currentBasePad = outPad

		if hasAudio {
			layerAudioPad := string(append(strconv.AppendInt([]byte("["), int64(inputIdx+1), 10), ":a]"...))
			aOutPad := "[a_chromium]"
			filterComplex.WriteString(layerAudioPad)
			filterComplex.WriteString(" aresample=async=1,aformat=sample_rates=48000:channel_layouts=stereo,asetpts=N,")
			filterComplex.WriteString(" volume@layer99=1.00 ")
			filterComplex.WriteString(aOutPad)
			filterComplex.WriteString(";\n")
			audioPads = append(audioPads, aOutPad)
			inputIdx += 2
		} else {
			inputIdx += 1
		}
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
		filterComplex.WriteString(":duration=longest:dropout_transition=0 [a_out];\n")
		finalAudioPad = "[a_out]"
	}

	finalVideoPad := "[v_out]"
	filterComplex.WriteString(currentBasePad)
	filterComplex.WriteString(" zmq=b=tcp\\\\://127.0.0.1\\\\:5555 ")
	filterComplex.WriteString(finalVideoPad)
	filterComplex.WriteString(";\n")

	return args, filterComplex.String(), finalVideoPad, finalAudioPad
}
