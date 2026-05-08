package engine

import "testing"

func TestIsSafeFilterValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Empty string", "", true},
		{"Basic alphanumeric", "Scale123", true},
		{"Allowed special characters", "iw*0.5:ih+10%/-_.", true},
		{"Contains comma", "100,200", false},
		{"Contains semicolon", "scale=100;crop=50", false},
		{"Contains square brackets", "[v0]", false},
		{"Contains equals", "scale=100", false},
		{"Contains single quote", "text'pwned", false},
		{"Contains double quote", "text\"pwned", false},
		{"Contains backslash", "path\\to\\file", false},
		{"Contains pipe", "a|b", false},
		{"Contains space", "scale 100", false},
		{"Contains exclamation mark", "scale!", false},
		{"Unsafe char at start", ",100", false},
		{"Unsafe char at end", "100,", false},
		{"Unicode character", "scaleπ", false},
		{"Non-ASCII control character", "scale\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeFilterValue(tt.input); got != tt.expected {
				t.Errorf("isSafeFilterValue(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
