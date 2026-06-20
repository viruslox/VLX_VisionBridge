package streamer

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

var destReplacer = strings.NewReplacer("\\", "\\\\", "|", "\\|")

// IsValidDestination strictly validates the destination URL to prevent FFmpeg tee muxer injection.
func IsValidDestination(dest string) bool {
	u, err := url.ParseRequestURI(dest)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "rtmp" && scheme != "rtmps" && scheme != "srt" {
		return false
	}

	if u.Host == "" {
		return false
	}

	if strings.ContainsAny(dest, "|\\\"'[]") {
		return false
	}

	return true
}

func BuildOutputArgs(cfg *models.Config) ([]string, error) {
	var args []string
	
	args = append(args, "!", "x264enc", "tune=zerolatency", "speed-preset=ultrafast", "!", "h264parse")

	if len(cfg.Output.Destinations) > 0 {
		args = append(args, "!", "tee", "name=t")
		for _, dest := range cfg.Output.Destinations {
			escaped := strings.ReplaceAll(dest, "\\", "\\\\")
			if strings.HasPrefix(strings.ToLower(dest), "srt://") {
				args = append(args, fmt.Sprintf("t. ! queue ! mpegtsmux ! srtsink uri=%s", escaped))
			} else {
				args = append(args, fmt.Sprintf("t. ! queue ! flvmux ! rtmpsink location=%s", escaped))
			}
		}
	}
	return args, nil
}
