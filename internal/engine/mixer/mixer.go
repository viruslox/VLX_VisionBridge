package mixer

import (
	"fmt"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	args = append(args,
		"compositor", "name=comp", "background=black", "!",
		"video/x-raw", "!", "videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!", "audioresample", "!",
		"avenc_aac", "!", "aacparse", "!", "tee", "name=atee",
	)

	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		"video/x-raw,width=1920,height=1080,framerate=30/1", "!", "comp.sink_0",
	)
	args = append(args,
		"audiotestsrc", "wave=silence", "is-live=true", "!",
		"audio/x-raw,rate=48000,channels=2", "!", "acomp.sink_0",
	)

	// Sources
	if cfg.Input.ChromiumSource.Active {
		// WebRTC Video
		args = append(args,
			"udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!",
			"rtpjitterbuffer", "latency=50", "!",
			"rtpvp8depay", "!", "vp8dec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!",
			"videoconvert", "!", "comp.sink_1",
		)
		// WebRTC Audio
		args = append(args,
			"udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!",
			"rtpjitterbuffer", "latency=50", "!",
			"rtpopusdepay", "!", "opusdec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!",
			"audioconvert", "!", "audioresample", "!", "acomp.sink_1",
		)
	} else if cfg.Input.MediaSource.Active {
		for i, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				srcName := fmt.Sprintf("src_%d", i)
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "name="+srcName)
				args = append(args, srcName+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!", "videoconvert", "!", fmt.Sprintf("comp.sink_%d", i+1))
				args = append(args, srcName+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "!", "audioconvert", "!", "audioresample", "!", fmt.Sprintf("acomp.sink_%d", i+1))
			}
		}
	}

	return args, "", "", ""
}
