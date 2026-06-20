package mixer

import (
	"fmt"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	args = append(args, "compositor", "name=comp", "sink_0::zorder=0", "sink_1::zorder=1", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "video/x-raw,format=RGBA", "!", "videoconvert", "!")

	args = append(args, "audiomixer", "name=acomp", "!")

	if cfg.Input.ChromiumSource.Active {
		args = append(args, "udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!", "rtpvp8depay", "!", "vp8dec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "comp.sink_1")
		args = append(args, "udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!", "rtpopusdepay", "!", "opusdec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "audioconvert", "!", "audioresample", "!", "acomp.sink_1")
	}

	if cfg.Input.MediaSource.Active {
		for i, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "name=src_"+fmt.Sprint(i))
				args = append(args, "src_"+fmt.Sprint(i)+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", fmt.Sprintf("comp.sink_%d", i))
				args = append(args, "src_"+fmt.Sprint(i)+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "audioconvert", "!", "audioresample", "!", fmt.Sprintf("acomp.sink_%d", i))
			}
		}
	}
	return args, "", "", ""
}
