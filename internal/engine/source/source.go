package source

import (
	"io/fs"
	"os"
	"path/filepath"
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

func BuildInputArgs(layer models.Layer) models.InputResult {
	safePath := SanitizeInputPath(layer.InputPath)
	inputType := strings.ToLower(layer.InputType)

	switch inputType {
	case "local":
		info, err := os.Stat(safePath)
		if err != nil || !info.IsDir() {
			return models.InputResult{Args: []string{"-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
		}

		var args []string
		hasVideo := false
		hasImage := false
		hasAudio := false
		var videos []string
		var images []string
		var audios []string

		filepath.WalkDir(safePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == ".mp4" || ext == ".webm" {
				hasVideo = true
				videos = append(videos, path)
			} else if ext == ".png" {
				hasImage = true
				images = append(images, path)
			} else if ext == ".mp3" {
				hasAudio = true
				audios = append(audios, path)
			}
			return nil
		})

		if hasVideo && !hasImage && !hasAudio && len(videos) > 0 {
			args = append(args, "-re", "-stream_loop", "-1", "-i", videos[0])
			return models.InputResult{Args: args, InputCount: 1, HasVideo: true, HasAudio: true}
		} else if hasImage && hasAudio && !hasVideo && len(images) > 0 && len(audios) > 0 {
			args = append(args, "-loop", "1", "-i", images[0], "-stream_loop", "-1", "-i", audios[0])
			return models.InputResult{Args: args, InputCount: 2, HasVideo: true, HasAudio: true}
		} else if hasImage && !hasVideo && !hasAudio && len(images) > 0 {
			args = append(args, "-re", "-loop", "1", "-i", images[0])
			return models.InputResult{Args: args, InputCount: 1, HasVideo: true, HasAudio: false}
		} else if hasAudio && !hasVideo && !hasImage && len(audios) > 0 {
			args = append(args, "-stream_loop", "-1", "-i", audios[0])
			return models.InputResult{Args: args, InputCount: 1, HasVideo: false, HasAudio: true}
		}
		return models.InputResult{Args: []string{"-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}

	case "ipc_audio":
		return models.InputResult{Args: []string{"-listen", "1", "-f", "s16le", "-ar", "48000", "-ac", "2", "-thread_queue_size", "1024", "-i", "unix:///tmp/vlx_audio.sock"}, InputCount: 1, HasVideo: false, HasAudio: true}
	case "srt":
		return models.InputResult{Args: []string{"-fflags", "nobuffer", "-flags", "low_delay", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "rtmp", "rtmps":
		return models.InputResult{Args: []string{"-listen", "1", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "webrtc":
		return models.InputResult{Args: []string{"-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "rtsp", "rtsps":
		return models.InputResult{Args: []string{"-rtsp_transport", "tcp", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	default:
		return models.InputResult{Args: []string{"-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	}
}
