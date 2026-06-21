package mixer

import (
	"fmt"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	// 1. RAMO VIDEO: Dal compositor all'encoder, fino al nodo "vtee"
	args = append(args,
		"compositor", "name=comp", "background=black", "!",
		"video/x-raw", "!", "videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	// 2. RAMO AUDIO: Dall'audiomixer all'encoder AAC, fino al nodo "atee"
	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!", "audioresample", "!",
		"avenc_aac", "!", "aacparse", "!", "tee", "name=atee",
	)

	// 3. MASTER CLOCK: Battito continuo per prevenire il freeze di GStreamer
	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		"video/x-raw,width=1920,height=1080,framerate=30/1", "!", "comp.sink_0",
	)

	// 4. SORGENTI MULTIMEDIALI
	if cfg.Input.ChromiumSource.Active {
		// WebRTC Video
		args = append(args,
			"udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!",
			"rtpvp8depay", "!", "vp8dec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!",
			"videoconvert", "!", "comp.sink_1",
		)
		// WebRTC Audio
		args = append(args,
			"udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!",
			"rtpopusdepay", "!", "opusdec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!",
			"audioconvert", "!", "audioresample", "!", "acomp.sink_1",
		)
	} else if cfg.Input.MediaSource.Active {
		for i, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				srcName := fmt.Sprintf("src_%d", i)
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "name="+srcName)
				// Collega dinamicamente le uscite hardware
				args = append(args, srcName+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!", "videoconvert", "!", fmt.Sprintf("comp.sink_%d", i+1))
				args = append(args, srcName+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!", "audioconvert", "!", "audioresample", "!", fmt.Sprintf("acomp.sink_%d", i+1))
			}
		}
	}

	return args, "", "", ""
}
