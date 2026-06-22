package mixer

import (
	"fmt"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	// 1. SINCRONIZZAZIONE PARAMETRI: Estrae i valori reali dal file settings JSON
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

	// 2. RAMO VIDEO: Applica la risoluzione e gli FPS scelti all'output principale
	args = append(args,
		"compositor", "name=comp", "background=black", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
		"videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "bitrate=8000", "key-int-max=60", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	// 3. RAMO AUDIO
	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!", "audioresample", "!",
		"avenc_aac", "bitrate=160000", "!", "aacparse", "!", "tee", "name=atee",
	)

	// 4. MASTER CLOCKS: Generati dinamicamente in base alle tue configurazioni
	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!", "comp.sink_0",
	)
	args = append(args,
		"audiotestsrc", "wave=silence", "is-live=true", "!",
		"audio/x-raw,rate=48000,channels=2", "!", "acomp.sink_0",
	)

	// 5. SORGENTI MULTIMEDIALI
	if cfg.Input.ChromiumSource.Active {
		// WebRTC Video
		args = append(args,
			"udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!",
			"rtpjitterbuffer", "latency=50", "!",
			"rtpvp8depay", "!", "vp8dec", "!", "queue", "!",
			// FIX "FUORI SCALA": Forza il resize alla risoluzione del config prima del compositor
			"videoscale", "!", "videorate", "!",
			fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
			"videoconvert", "!", "comp.sink_1",
		)
		// WebRTC Audio
		args = append(args,
			"udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!",
			"rtpjitterbuffer", "latency=50", "!",
			"rtpopusdepay", "!", "opusdec", "!", "queue", "!",
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
