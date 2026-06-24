package mixer

import (
	"fmt"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

// BuildFilterComplex constructs a static, immutable GStreamer pipeline.
// It captures the X11 virtual display and the isolated PulseAudio sink.
func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	resParts := strings.Split(cfg.Input.Resolution, "x")
	resWidth, resHeight := "1920", "1080"
	if len(resParts) == 2 {
		resWidth = resParts[0]
		resHeight = resParts[1]
	}

	framerate := "30/1"
	if cfg.Input.Framerate > 0 {
		framerate = fmt.Sprintf("%d/1", cfg.Input.Framerate)
	}

	var args []string

	// Video pipeline: Compositor -> Scale/Rate -> x264enc (8Mbps, Keyframe interval: 1s)
	args = append(args,
		"compositor", "name=comp", "background=black", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
		"videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "bitrate=8000", "key-int-max=30", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	// Audio pipeline: Mixer -> Resample (48kHz) -> AAC
	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!",
		"audioresample", "!",
		"audio/x-raw,rate=48000,channels=2", "!",
		"avenc_aac", "bitrate=160000", "!",
		"aacparse", "!", "tee", "name=atee",
	)

	// Master Clocks: Ensures continuous streaming even if sources drop frames or go silent
	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!", "comp.sink_0",
	)
	args = append(args,
		"audiotestsrc", "wave=silence", "is-live=true", "!",
		"audio/x-raw,rate=48000,channels=2", "!", "acomp.sink_0",
	)

	// Native input capture (Chromium via X11 and PulseAudio)
	if cfg.Input.ChromiumSource.Active {
		args = append(args,
			"ximagesrc", "display-name=:99", "use-damage=0", "show-pointer=false", "!",
			"videoscale", "!", "videorate", "!",
			fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
			"videoconvert", "!", "comp.sink_1",
		)
		args = append(args,
			"pulsesrc", "device=VisionBridgeSink.monitor", "!",
			"audioconvert", "!", "audioresample", "!", "acomp.sink_1",
		)
	}

	return args, "", "", ""
}
