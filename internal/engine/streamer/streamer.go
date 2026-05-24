package streamer

import (
	"fmt"
	"net/url"
	"strconv"
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
	if cfg.Output.Resolution != "" {
		args = append(args, "-s", cfg.Output.Resolution)
	}
	if cfg.Output.FPS > 0 {
		args = append(args, "-r", strconv.Itoa(cfg.Output.FPS))
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p")
	if cfg.Output.VideoBitrate != "" {
		args = append(args, "-b:v", cfg.Output.VideoBitrate, "-maxrate", cfg.Output.VideoBitrate, "-bufsize", cfg.Output.VideoBitrate)
	}

	if cfg.Output.AudioBitrate != "" {
		args = append(args, "-c:a", "aac", "-b:a", cfg.Output.AudioBitrate)
	} else {
		args = append(args, "-c:a", "aac")
	}
	
	args = append(args, "-flags", "+global_header")
	if len(cfg.Output.Destinations) > 0 {
		var teeDestinations []string
		for _, dest := range cfg.Output.Destinations {
		    if !IsValidDestination(dest) {
		        return nil, fmt.Errorf("invalid or unsafe output destination: %s", dest)
		    }
		    escaped := destReplacer.Replace(dest)
		    if strings.HasPrefix(strings.ToLower(dest), "srt://") {
		        teeDestinations = append(teeDestinations, "[f=mpegts:onfail=ignore]"+escaped)
		    } else {
		        teeDestinations = append(teeDestinations, "[f=flv:onfail=ignore]"+escaped)
		    }
		}
		teeMap := strings.Join(teeDestinations, "|")
		args = append(args, "-f", "tee", "-use_fifo", "1", "-fifo_options", "drop_pkts_on_overflow=1:attempt_recovery=1:recovery_wait_time=1", teeMap)
	}
	return args, nil
}
