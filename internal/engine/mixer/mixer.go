package mixer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

// BuildFilterComplex constructs a static, immutable GStreamer pipeline dynamically driven by configuration.
func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	// Parse Video Resolution
	resStr := cfg.Input.Resolution
	if cfg.Output.Resolution != "" {
		resStr = cfg.Output.Resolution
	}
	resParts := strings.Split(resStr, "x")
	resWidth, resHeight := "1920", "1080"
	if len(resParts) == 2 {
		resWidth = resParts[0]
		resHeight = resParts[1]
	}

	// Parse Video Framerate
	framerate := "30/1"
	if cfg.Output.FPS > 0 {
		framerate = fmt.Sprintf("%d/1", cfg.Output.FPS)
	} else if cfg.Input.Framerate > 0 {
		framerate = fmt.Sprintf("%d/1", cfg.Input.Framerate)
	}

	// Parse Audio Sample Rate (Default to 44100 to fix RTMP conflicts)
	sampleRate := 44100
	if cfg.Output.AudioSampleRate > 0 {
		sampleRate = cfg.Output.AudioSampleRate
	}

	// Parse Video Bitrate (e.g., "8000k" -> "8000")
	vBitrate := strings.TrimSuffix(cfg.Output.VideoBitrate, "k")
	if vBitrate == "" {
		vBitrate = "8000"
	}

	// Parse Audio Bitrate (e.g., "160k" -> 160000 bits)
	aBitrateStr := strings.TrimSuffix(cfg.Output.AudioBitrate, "k")
	aBitrateInt := 160000
	if val, err := strconv.Atoi(aBitrateStr); err == nil {
		aBitrateInt = val * 1000
	}

	var args []string

	// Video pipeline: Compositor -> Scale/Rate -> x264enc
	args = append(args,
	    "compositor", "name=comp", "background=black", "!",
	    fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!",
	    "videoconvert", "!",
	    "video/x-raw,format=I420,colorimetry=bt709", "!", // <--- BT.709 - LIMITED RANGE
	    "x264enc", "tune=zerolatency", "speed-preset=ultrafast", fmt.Sprintf("bitrate=%s", vBitrate), "key-int-max=60", "!", // <--- 2secs KEYFRAME
	    "h264parse", "!", "tee", "name=vtee",
	)

	// Audio pipeline: Mixer -> Resample -> AAC
	args = append(args,
		"audiomixer", "name=acomp", "!",
		"audioconvert", "!",
		"audioresample", "!",
		fmt.Sprintf("audio/x-raw,rate=%d,channels=2", sampleRate), "!",
		"avenc_aac", fmt.Sprintf("bitrate=%d", aBitrateInt), "!",
		"aacparse", "!", "tee", "name=atee",
	)

	// Master Clocks: Ensures continuous streaming even if Chromium drops
	args = append(args,
		"videotestsrc", "pattern=black", "is-live=true", "!",
		fmt.Sprintf("video/x-raw,width=%s,height=%s,framerate=%s", resWidth, resHeight, framerate), "!", "comp.sink_0",
	)
	args = append(args,
		"audiotestsrc", "wave=silence", "is-live=true", "!",
		fmt.Sprintf("audio/x-raw,rate=%d,channels=2", sampleRate), "!", "acomp.sink_0",
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
