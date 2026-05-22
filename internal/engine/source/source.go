package source

import (
	"strings"

	"github.com/user/VLX_VisionBridge/internal/models"
)

// SanitizeInputPath prevents FFmpeg argument injection by prefixing local paths
// that start with a dash ("-") with "./".
func SanitizeInputPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "-") {
		return "./" + path
	}
	return path
}

func BuildInputArgs(layer models.Layer) []string {
	safePath := SanitizeInputPath(layer.InputPath)
	switch layer.InputType {
	case "folder":
		return []string{"-f", "image2", "-loop", "1", "-i", safePath}
	case "loop":
		return []string{"-stream_loop", "-1", "-i", safePath}
	case "srt":
		return []string{"-fflags", "nobuffer", "-flags", "low_delay", "-i", safePath}
	default:
		return []string{"-i", safePath}
	}
}
