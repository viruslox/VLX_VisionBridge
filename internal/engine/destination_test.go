package engine

import "testing"

func TestIsValidDestination(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		expected bool
	}{
		// Happy paths
		{"Valid RTMP", "rtmp://live.twitch.tv/app/live_xyz", true},
		{"Valid RTMPS", "rtmps://live-api-s.facebook.com:443/rtmp/", true},
		{"Valid SRT", "srt://example.com:1234", true},
		{"Valid RTMP with params", "rtmp://localhost/app/stream?key=value", true},

		// Invalid schemes
		{"HTTP scheme", "http://example.com", false},
		{"HTTPS scheme", "https://example.com", false},
		{"UDP scheme", "udp://1.2.3.4:1234", false},
		{"No scheme", "example.com/live", false},

		// Edge cases
		{"Empty string", "", false},
		{"Malformed URL", "rtmp:// space in host", false},
		{"Missing host", "rtmp:///path", false},
		{"Missing host and path", "srt://", false},

		// Injection attempts
		{"Pipe injection", "rtmp://localhost/app/stream|[f=mp4]/tmp/pwned.mp4", false},
		{"Backslash injection", "rtmp://localhost/app/stream\\inject", false},
		{"Double quote injection", "rtmp://localhost/app/stream\"inject", false},
		{"Single quote injection", "rtmp://localhost/app/stream'inject", false},
		{"Left bracket injection", "rtmp://localhost/app/stream[inject", false},
		{"Right bracket injection", "rtmp://localhost/app/stream]inject", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidDestination(tt.dest); got != tt.expected {
				t.Errorf("isValidDestination(%q) = %v, want %v", tt.dest, got, tt.expected)
			}
		})
	}
}
