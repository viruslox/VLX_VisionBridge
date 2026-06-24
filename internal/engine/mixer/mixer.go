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

	// STATIC VIDEO PIPELINE: Capture Xvfb display :99
	args = append(args,
		"ximagesrc", "display-name=:99", "use-damage=0", "show-pointer=false", "!",
		"videoscale", "!", "videorate", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
		"videoconvert", "!", "comp.sink_0",
	)

	// STATIC AUDIO PIPELINE: Capture isolated PulseAudio server
	args = append(args,
		"pulsesrc", "device=VisionBridgeSink.monitor", "!",
		"audioconvert", "!", "audioresample", "!", "acomp.sink_0",
	)

	if cfg.Input.ChromiumSource.Active {
		args = append(args, "udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!", "rtpvp8depay", "!", "vp8dec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "comp.sink_1")
		args = append(args, "udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!", "rtpopusdepay", "!", "opusdec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "audioconvert", "!", "audioresample", "!", "acomp.sink_1")
	}

	args = append(args,
		"compositor", "name=comp", "sink_0::zorder=0", "sink_1::zorder=1", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "video/x-raw,format=I420", "!", "videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "bitrate=8000", "key-int-max=30", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	args = append(args,
		"audiomixer", "name=acomp", "!", "audioconvert", "!", "audioresample", "!",
		"avenc_aac", "bitrate=160000", "!", "aacparse", "!", "tee", "name=atee",
	)

	return args, "", "", ""
}
