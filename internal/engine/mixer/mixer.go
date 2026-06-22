package mixer

import (
	"fmt"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	resParts := strings.Split(cfg.Input.Resolution, "x")
	resWidth := "1920"
	resHeight := "1080"
	if len(resParts) == 2 {
		resWidth = resParts[0]
		resHeight = resParts[1]
	}

	framerate := "30/1"
	if cfg.Input.Framerate > 0 {
		framerate = fmt.Sprintf("%d/1", cfg.Input.Framerate)
	}

	// RAMO VIDEO
	args = append(args,
		"compositor", "name=comp", "background=black", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
		"videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "bitrate=8000", "key-int-max=60", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	// RAMO AUDIO
	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!", "audioresample", "!",
		"avenc_aac", "bitrate=160000", "!", "aacparse", "!", "tee", "name=atee",
	)

	// MASTER CLOCKS
	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!", "comp.sink_0",
	)
	args = append(args,
		"audiotestsrc", "wave=silence", "is-live=true", "!",
		"audio/x-raw,rate=48000,channels=2", "!", "acomp.sink_0",
	)

	if cfg.Input.ChromiumSource.Active {
		// X11 NATIVE CAPTURE: Perfetto per Chromium Headless
		args = append(args,
			"ximagesrc", "display-name=:99", "use-damage=0", "show-pointer=false", "!",
			"videoscale", "!", "videorate", "!",
			fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
			"videoconvert", "!", "comp.sink_1",
		)
		// PULSEAUDIO NATIVE CAPTURE: Dal server virtuale isolato
		args = append(args,
			"pulsesrc", "device=VisionBridgeSink.monitor", "!",
			"audioconvert", "!", "audioresample", "!", "acomp.sink_1",
		)
	} else if cfg.Input.MediaSource.Active {
		for i, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				srcName := fmt.Sprintf("src_%d", i)
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "name="+srcName)
				args = append(args, srcName+".", "!", "queue", "!", 
					"videoscale", "!", "videorate", "!",
					fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
					"videoconvert", "!", fmt.Sprintf("comp.sink_%d", i+1))
				args = append(args, srcName+".", "!", "queue", "!", "audioconvert", "!", "audioresample", "!", fmt.Sprintf("acomp.sink_%d", i+1))
			}
		}
	}

	return args, "", "", ""
}
