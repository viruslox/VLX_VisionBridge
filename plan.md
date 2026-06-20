1. **Fix `models.Config`:**
   - Use `replace_with_git_merge_diff` on `internal/models/models.go`:
     ```go
     <<<<<<< SEARCH
     type FFmpegSource struct {
	Active bool    `yaml:"active"`
	Layers []Layer `yaml:"layers"`
     }
     =======
     type MediaSource struct {
	Active bool    `yaml:"active"`
	Layers []Layer `yaml:"layers"`
     }
     >>>>>>> REPLACE
     <<<<<<< SEARCH
     type InputSettings struct {
	BgColor        string         `yaml:"bg_color" json:"bg_color"`
	Resolution     string         `yaml:"resolution"`
	Framerate      int            `yaml:"framerate" json:"framerate"`
	FFmpegSource   FFmpegSource   `yaml:"ffmpeg_source"`
	ChromiumSource ChromiumSource `yaml:"chromium_source"`
     }
     =======
     type InputSettings struct {
	BgColor        string         `yaml:"bg_color" json:"bg_color"`
	Resolution     string         `yaml:"resolution"`
	Framerate      int            `yaml:"framerate" json:"framerate"`
	WebrtcPortMin  int            `yaml:"webrtc_port_min"`
	WebrtcPortMax  int            `yaml:"webrtc_port_max"`
	MediaSource    MediaSource    `yaml:"media_source"`
	ChromiumSource ChromiumSource `yaml:"chromium_source"`
     }
     >>>>>>> REPLACE
     ```
   - Verify by running `cat internal/models/models.go`.

2. **Fix `internal/engine/manager.go` references:**
   - Use `run_in_bash_session` to run `sed -i 's/FFmpegSource/MediaSource/g' internal/engine/manager.go`.
   - Use `run_in_bash_session` to run `sed -i 's/\[ffmpeg_source\]/\[media_source\]/g' internal/engine/manager.go`.
   - Verify by running `go build ./...`.

3. **Fix `internal/engine/mixer/mixer.go` references & Pipeline Logic:**
   - Use `replace_with_git_merge_diff` on `internal/engine/mixer/mixer.go`:
     ```go
     <<<<<<< SEARCH
     func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	args = append(args, "compositor", "name=comp", "sink_0::zorder=0", "sink_1::zorder=1", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "video/x-raw,format=I420")

	if cfg.Input.ChromiumSource.Active {
		args = append(args, "appsrc", "name=webrtc_video", "format=time", "is-live=true", "do-timestamp=true", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "comp.sink_1")
	}

	if cfg.Input.FFmpegSource.Active {
		for i, layer := range cfg.Input.FFmpegSource.Layers {
			if layer.Active {
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", fmt.Sprintf("comp.sink_%d", i))
			}
		}
	}
	return args, "", "", ""
     }
     =======
     func BuildFilterComplex(cfg *models.Config) ([]string, string, string, string) {
	var args []string

	args = append(args, "compositor", "name=comp", "sink_0::zorder=0", "sink_1::zorder=1", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "video/x-raw,format=RGBA", "!", "videoconvert", "!")

	args = append(args, "audiomixer", "name=acomp", "!")

	if cfg.Input.ChromiumSource.Active {
		args = append(args, "udpsrc", "port=50002", "caps=application/x-rtp,media=video,clock-rate=90000,encoding-name=VP8", "!", "rtpvp8depay", "!", "vp8dec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", "comp.sink_1")
		args = append(args, "udpsrc", "port=50003", "caps=application/x-rtp,media=audio,clock-rate=48000,encoding-name=OPUS", "!", "rtpopusdepay", "!", "opusdec", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "audioconvert", "!", "audioresample", "!", "acomp.sink_1")
	}

	if cfg.Input.MediaSource.Active {
		for i, layer := range cfg.Input.MediaSource.Layers {
			if layer.Active {
				args = append(args, "uridecodebin", "uri="+layer.InputPath, "name=src_"+fmt.Sprint(i))
				args = append(args, "src_"+fmt.Sprint(i)+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "videoconvert", "!", fmt.Sprintf("comp.sink_%d", i))
				args = append(args, "src_"+fmt.Sprint(i)+".", "!", "queue", "leaky=downstream", "max-size-buffers=1", "max-size-time=30000000", "!", "audioconvert", "!", "audioresample", "!", fmt.Sprintf("acomp.sink_%d", i))
			}
		}
	}
	return args, "", "", ""
     }
     >>>>>>> REPLACE
     ```
   - Verify by running `go build ./...`.

4. **Fix WebRTC to GStreamer Bridge (`internal/engine/webrtc.go`):**
   - Use `replace_with_git_merge_diff` on `internal/engine/webrtc.go`:
     ```go
     <<<<<<< SEARCH
     import (
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v3"
     )
     =======
     import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v3"
     )
     >>>>>>> REPLACE
     <<<<<<< SEARCH
     func handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
     =======
     func handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
     >>>>>>> REPLACE
     <<<<<<< SEARCH
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Got WebRTC Track: %s (%s)", track.Kind().String(), track.Codec().MimeType)
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			WebRTCVideoTrack = track
		} else if track.Kind() == webrtc.RTPCodecTypeAudio {
			WebRTCAudioTrack = track
		}
	})
     =======
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Got WebRTC Track: %s (%s)", track.Kind().String(), track.Codec().MimeType)

		port := 50002
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			port = 50003
		}

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			log.Printf("Failed to resolve UDP addr: %v", err)
			return
		}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			log.Printf("Failed to dial UDP: %v", err)
			return
		}
		defer conn.Close()

		b := make([]byte, 1500)
		for {
			n, _, readErr := track.Read(b)
			if readErr != nil {
				return
			}
			if _, writeErr := conn.Write(b[:n]); writeErr != nil {
				return
			}
		}
	})
     >>>>>>> REPLACE
     ```
   - Verify by running `cat internal/engine/webrtc.go` and `go build ./...`.

5. **Test the solution:**
   - Run `go test ./...`.

6. **Pre-commit:**
   - Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.

7. **Submit.**
