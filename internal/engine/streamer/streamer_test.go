package streamer

import (
	"reflect"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestBuildOutputArgs(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *models.Config
		expected    []string
		expectError bool
	}{
		{
			name: "Minimal config",
			cfg: &models.Config{
				Output: models.OutputSettings{},
			},
			expected: []string{
				"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-pix_fmt", "yuv420p",
				"-c:a", "aac",
				"-flags", "+global_header",
			},
			expectError: false,
		},
		{
			name: "Full config with multiple destinations",
			cfg: &models.Config{
				Output: models.OutputSettings{
					Resolution:   "1920x1080",
					FPS:          60,
					VideoBitrate: "6000k",
					AudioBitrate: "160k",
					Destinations: []string{
						"rtmp://live.twitch.tv/app/live_xyz",
						"srt://example.com:1234",
						"rtmps://live-api-s.facebook.com:443/rtmp/",
					},
				},
			},
			expected: []string{
				"-s", "1920x1080",
				"-r", "60",
				"-fps_mode", "cfr",
				"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-pix_fmt", "yuv420p",
				"-b:v", "6000k", "-maxrate", "6000k", "-bufsize", "6000k",
				"-c:a", "aac", "-b:a", "160k",
				"-flags", "+global_header",
				"-f", "tee", "-use_fifo", "1", "-fifo_options", "drop_pkts_on_overflow=1:attempt_recovery=1:recovery_wait_time=1",
				"[f=flv:onfail=ignore]rtmp://live.twitch.tv/app/live_xyz|[f=mpegts:onfail=ignore]srt://example.com:1234|[f=flv:onfail=ignore]rtmps://live-api-s.facebook.com:443/rtmp/",
			},
			expectError: false,
		},
		{
			name: "Invalid destination format",
			cfg: &models.Config{
				Output: models.OutputSettings{
					Destinations: []string{
						"rtmp://live.twitch.tv/app/live_xyz",
						"http://example.com/invalid",
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildOutputArgs(tt.cfg)
			if (err != nil) != tt.expectError {
				t.Errorf("BuildOutputArgs() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !tt.expectError && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("BuildOutputArgs() got = %v, want %v", got, tt.expected)
			}
		})
	}
}
