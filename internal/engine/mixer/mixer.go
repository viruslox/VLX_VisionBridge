package mixer

import (
	"fmt"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/engine/source"
	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string
	var filterComplex strings.Builder

	bgColor := cfg.Input.BgColor
	if bgColor == "" {
		bgColor = "black"
	}
	filterComplex.WriteString(fmt.Sprintf("color=s=%s:r=30:c=%s [base];\n", cfg.Input.Resolution, bgColor))

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
				layerVideoPad = fmt.Sprintf("[%d:v]", inputIdx)
				layerAudioPad = fmt.Sprintf("[%d:a]", inputIdx)
			} else if res.InputCount == 2 {
				layerVideoPad = fmt.Sprintf("[%d:v]", inputIdx)
				layerAudioPad = fmt.Sprintf("[%d:a]", inputIdx+1)
			}

			// Handle Video
			if (media == "Video" || media == "Video+Audio") && res.HasVideo {
				scaledPad := fmt.Sprintf("[v%d_scaled]", i)
				outPad := fmt.Sprintf("[out%d]", i)

				filterComplex.WriteString(fmt.Sprintf("%s setpts=PTS-STARTPTS,scale=%d:-1 %s;\n", layerVideoPad, layer.Size, scaledPad))
				filterComplex.WriteString(fmt.Sprintf("%s%s overlay@layer%d=x=%d:y=%d %s;\n", currentBasePad, scaledPad, layer.ID, layer.X, layer.Y, outPad))

				currentBasePad = outPad
			}

			// Handle Audio
			if (media == "Audio" || media == "Video+Audio") && res.HasAudio {
				aOutPad := fmt.Sprintf("[a%d]", i)

				volumeVal := 1.0
				if layer.Volume != nil {
					volumeVal = float64(*layer.Volume) / 100.0
				}

				filterComplex.WriteString(fmt.Sprintf("%s aresample=48000:async=1,aformat=sample_rates=48000:channel_layouts=stereo,asetpts=PTS-STARTPTS, volume@layer%d=%.2f %s;\n", layerAudioPad, layer.ID, volumeVal, aOutPad))

				audioPads = append(audioPads, aOutPad)
			}

			inputIdx += res.InputCount
		}
	}

	if cfg.Input.ChromiumSource.Active {
		cs := cfg.Input.ChromiumSource
		
		hasAudio := cs.Z1Active || cs.Z2Active || cs.Z3Active || cs.Z4Active ||
			cs.Z5Active || cs.Z6Active || cs.Z7Active || cs.Z8Active

		// OPTIMIZATION: Aggiunto -framerate fisso e -use_wallclock_as_timestamps per azzerare il delay ed evitare audio gracchiante
		if hasAudio {
			args = append(args, 
				"-f", "x11grab", 
				"-thread_queue_size", "4096", 
				"-framerate", "30", 
				"-video_size", cfg.Input.Resolution, 
				"-draw_mouse", "0", 
				"-use_wallclock_as_timestamps", "1", 
				"-i", ":99", 
				"-f", "pulse", 
				"-thread_queue_size", "4096", 
				"-use_wallclock_as_timestamps", "1", 
				"-i", "vlx_chromium_sink.monitor",
			)
		} else {
			args = append(args, 
				"-f", "x11grab", 
				"-thread_queue_size", "4096", 
				"-framerate", "30", 
				"-video_size", cfg.Input.Resolution, 
				"-draw_mouse", "0", 
				"-use_wallclock_as_timestamps", "1", 
				"-i", ":99",
			)
		}

		chromaColor := "0x00FF00"

		layerVideoPad := fmt.Sprintf("[%d:v]", inputIdx)
		chromaPad := "[chroma_chromium]"
		outPad := "[out_chromium]"

		filterComplex.WriteString(fmt.Sprintf("%s setpts=PTS-STARTPTS,colorkey=%s:0.1:0.1 %s;\n", layerVideoPad, chromaColor, chromaPad))
		filterComplex.WriteString(fmt.Sprintf("%s%s overlay=x=0:y=0 %s;\n", currentBasePad, chromaPad, outPad))

		currentBasePad = outPad

		if hasAudio {
			layerAudioPad := fmt.Sprintf("[%d:a]", inputIdx+1)
			aOutPad := "[a_chromium]"

			filterComplex.WriteString(fmt.Sprintf("%s aresample=48000:async=1,aformat=sample_rates=48000:channel_layouts=stereo,asetpts=PTS-STARTPTS, volume@layer99=1.00 %s;\n", layerAudioPad, aOutPad))

			audioPads = append(audioPads, aOutPad)
			inputIdx += 2
		} else {
			inputIdx += 1
		}
	}

	var finalAudioPad string
	if len(audioPads) == 0 {
		filterComplex.WriteString("anullsrc=r=48000:cl=stereo [a_out];\n")
		finalAudioPad = "[a_out]"
	} else if len(audioPads) == 1 {
		finalAudioPad = audioPads[0]
	} else {
		for _, pad := range audioPads {
			filterComplex.WriteString(pad)
		}
		filterComplex.WriteString(fmt.Sprintf(" amix=inputs=%d:duration=longest:dropout_transition=0 [a_out];\n", len(audioPads)))
		finalAudioPad = "[a_out]"
	}

	finalVideoPad := "[v_out]"
	filterComplex.WriteString(fmt.Sprintf("%s zmq=b=tcp\\\\://127.0.0.1\\\\:5555 %s;\n", currentBasePad, finalVideoPad))

	return args, filterComplex.String(), finalVideoPad, finalAudioPad
}
