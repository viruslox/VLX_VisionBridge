package source

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func generateSpacer(path string, delaySec int) error {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi", "-i", "color=c=black:s=1920x1080:r=30",
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000",
		"-c:v", "libx264",
		"-t", strconv.Itoa(delaySec),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		path,
	)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate spacer: %w", err)
	}
	return nil
}

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
			if layer.FolderOptions.IsFolder {
				if layer.FolderOptions.Shuffle {
					rand.Seed(time.Now().UnixNano())
					rand.Shuffle(len(videos), func(i, j int) {
						videos[i], videos[j] = videos[j], videos[i]
					})
				}

				playlistPath := fmt.Sprintf("/tmp/vlx_playlist_%d.txt", layer.ID)
				playlistFile, err := os.Create(playlistPath)
				if err != nil {
					// Fallback to single video play if we can't create the playlist
					args = append(args, "-re", "-stream_loop", "-1", "-i", videos[0])
					return models.InputResult{Args: args, InputCount: 1, HasVideo: true, HasAudio: true}
				}
				defer playlistFile.Close()

				var spacerPath string
				var spacerErr error
				if layer.FolderOptions.DelaySec > 0 {
					spacerPath = fmt.Sprintf("/tmp/vlx_spacer_%d.mp4", layer.ID)
					spacerErr = generateSpacer(spacerPath, layer.FolderOptions.DelaySec)
				}

				for _, video := range videos {
					playlistFile.WriteString(fmt.Sprintf("file '%s'\n", video))
					if layer.FolderOptions.DelaySec > 0 && spacerErr == nil {
						playlistFile.WriteString(fmt.Sprintf("file '%s'\n", spacerPath))
					}
				}

				args = append(args, "-re")
				if layer.FolderOptions.Loop {
					args = append(args, "-stream_loop", "-1")
				}
				args = append(args, "-f", "concat", "-safe", "0", "-i", playlistPath)
				return models.InputResult{Args: args, InputCount: 1, HasVideo: true, HasAudio: true}
			} else {
				args = append(args, "-re", "-stream_loop", "-1", "-i", videos[0])
				return models.InputResult{Args: args, InputCount: 1, HasVideo: true, HasAudio: true}
			}
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
		sockPath := safePath
		if sockPath == "" {
			sockPath = "/tmp/vlx_audio.sock"
		}
		return models.InputResult{Args: []string{"-listen", "1", "-f", "s16le", "-ar", "48000", "-ac", "2", "-thread_queue_size", "1024", "-i", "unix://" + sockPath}, InputCount: 1, HasVideo: false, HasAudio: true}
	case "srt":
		return models.InputResult{Args: []string{"-fflags", "nobuffer", "-flags", "low_delay", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "rtmp", "rtmps":
		return models.InputResult{Args: []string{"-fflags", "nobuffer", "-flags", "low_delay", "-listen", "1", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "webrtc":
		return models.InputResult{Args: []string{"-fflags", "nobuffer", "-flags", "low_delay", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	case "rtsp", "rtsps":
		return models.InputResult{Args: []string{"-fflags", "nobuffer", "-flags", "low_delay", "-rtsp_transport", "tcp", "-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	default:
		return models.InputResult{Args: []string{"-i", safePath}, InputCount: 1, HasVideo: true, HasAudio: true}
	}
}
