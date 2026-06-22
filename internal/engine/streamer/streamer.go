package streamer

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func IsValidDestination(dest string) bool {
	u, err := url.ParseRequestURI(dest)
	if err != nil {
		return false
	}
	// Ora accetteremo principalmente URL locali semplici come rtmp://127.0.0.1:1935/live/...
	scheme := strings.ToLower(u.Scheme)
	if scheme != "rtmp" && scheme != "rtmps" && scheme != "srt" {
		return false
	}
	return true
}

func BuildOutputArgs(cfg *models.Config) ([]string, error) {
	var args []string

	if len(cfg.Output.Destinations) == 0 {
		args = append(args, "vtee.", "!", "fakesink", "atee.", "!", "fakesink")
		return args, nil
	}

	for i, dest := range cfg.Output.Destinations {
		if !IsValidDestination(dest) {
			return nil, fmt.Errorf("invalid destination URL: %s", dest)
		}
		
		escaped := strings.ReplaceAll(dest, "\\", "\\\\")
		muxName := fmt.Sprintf("mux%d", i)

		if strings.HasPrefix(strings.ToLower(dest), "srt://") {
			args = append(args, "mpegtsmux", "name="+muxName)
			args = append(args, "vtee.", "!", "queue", "!", muxName+".")
			args = append(args, "atee.", "!", "queue", "!", muxName+".")
			args = append(args, muxName+".", "!", "srtsink", "uri="+escaped)
		} else {
			// Output RTMP standard. Il nodo rtmpsink ora punterà al MediaMTX locale in chiaro.
			args = append(args, "flvmux", "name="+muxName, "streamable=true")
			args = append(args, "vtee.", "!", "queue", "!", muxName+".video")
			args = append(args, "atee.", "!", "queue", "!", muxName+".audio")
			args = append(args, muxName+".", "!", "rtmpsink", "location="+escaped)
		}
	}

	return args, nil
}
