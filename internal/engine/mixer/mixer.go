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
		"videoconvert", "!",
		"x264enc", "tune=zerolatency", "speed-preset=ultrafast", "bitrate=8000", "key-int-max=30", "!",
		"h264parse", "!", "tee", "name=vtee",
	)

	// STATIC AUDIO PIPELINE: Capture isolated PulseAudio server
	args = append(args,
		"pulsesrc", "device=VisionBridgeSink.monitor", "!",
		"audioconvert", "!", "audioresample", "!",
		"avenc_aac", "bitrate=160000", "!", "aacparse", "!", "tee", "name=atee",
	)

	return args, "", "", ""
}
