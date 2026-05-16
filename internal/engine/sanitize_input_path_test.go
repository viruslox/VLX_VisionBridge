package engine

import "testing"

func TestSanitizeInputPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Normal path", "video.mp4", "video.mp4"},
		{"Path starting with slash", "/tmp/video.mp4", "/tmp/video.mp4"},
		{"Path starting with dot", "./video.mp4", "./video.mp4"},
		{"Path starting with dash", "-vcodec", "./-vcodec"},
		{"Path starting with dash and extension", "-video.mp4", "./-video.mp4"},
		{"URL with scheme", "rtmp://server/live", "rtmp://server/live"},
		{"URL with scheme starting with dash", "-rtmp://server/live", "./-rtmp://server/live"},
		{"Fake URL with injected arg", "-i://example.com", "./-i://example.com"},
		{"Dash in the middle", "video-file.mp4", "video-file.mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeInputPath(tt.input); got != tt.expected {
				t.Errorf("sanitizeInputPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
