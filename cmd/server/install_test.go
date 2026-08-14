package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseEligibleUsers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "Happy path with eligible users",
			input: `root:x:0:0:root:/root:/bin/bash
user1:x:1000:1000:User One,,,:/home/user1:/bin/bash
user2:x:1001:1001:User Two,,,:/home/user2:/bin/zsh
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
`,
			expected: []string{"user1", "user2"},
		},
		{
			name: "Invalid shell for UID >= 1000",
			input: `user1:x:1000:1000:User One,,,:/home/user1:/bin/false
user2:x:1001:1001:User Two,,,:/home/user2:/usr/sbin/nologin
`,
			expected: nil,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: nil,
		},
		{
			name: "Invalid format (less than 7 fields)",
			input: `root:x:0:0:root:/root
user1:x:1000:1000:User One
`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEligibleUsers([]byte(tt.input))
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseEligibleUsers() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCopyTemplate(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Create new file", func(t *testing.T) {
		destPath := filepath.Join(tempDir, "new_visionbridge.settings")
		tmplContent := []byte("a: 1\nb: 2\n")

		err := copyTemplate(tmplContent, destPath)
		if err != nil {
			t.Fatalf("copyTemplate failed: %v", err)
		}

		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("Failed to read dest file: %v", err)
		}

		if string(content) != "a: 1\nb: 2\n" {
			t.Errorf("Expected content 'a: 1\\nb: 2\\n', got '%s'", string(content))
		}
	})

	t.Run("Merge with existing file", func(t *testing.T) {
		destPath := filepath.Join(tempDir, "existing_visionbridge.settings")
		existingContent := []byte("a: 10\n")
		err := os.WriteFile(destPath, existingContent, 0600)
		if err != nil {
			t.Fatalf("Failed to write existing file: %v", err)
		}

		tmplContent := []byte("a: 1\nb: 2\n")
		err = copyTemplate(tmplContent, destPath)
		if err != nil {
			t.Fatalf("copyTemplate failed: %v", err)
		}

		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("Failed to read dest file: %v", err)
		}

		expected := "a: 10\nb: 2\n"
		if string(content) != expected {
			t.Errorf("Expected content '%s', got '%s'", expected, string(content))
		}
	})

	t.Run("Invalid YAML template", func(t *testing.T) {
		destPath := filepath.Join(tempDir, "invalid_tmpl.yaml")
		existingContent := []byte("a: 10\n")
		os.WriteFile(destPath, existingContent, 0600)

		tmplContent := []byte("invalid:\n  - yaml\n- content")
		err := copyTemplate(tmplContent, destPath)
		if err == nil {
			t.Errorf("Expected error for invalid template YAML, got nil")
		}
	})

	t.Run("Invalid existing YAML", func(t *testing.T) {
		destPath := filepath.Join(tempDir, "invalid_dest.yaml")
		existingContent := []byte("invalid:\n  - yaml\n- content")
		os.WriteFile(destPath, existingContent, 0600)

		tmplContent := []byte("a: 1\n")
		err := copyTemplate(tmplContent, destPath)
		if err == nil {
			t.Errorf("Expected error for invalid existing YAML, got nil")
		}
	})

	t.Run("ReadFile error", func(t *testing.T) {
		destPath := filepath.Join(tempDir, "is_a_dir")
		err := os.Mkdir(destPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		tmplContent := []byte("a: 1\n")
		err = copyTemplate(tmplContent, destPath)
		if err == nil {
			t.Errorf("Expected error when reading a directory, got nil")
		}
	})
}

func TestAddMissingKeys(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		tmpl     string
		expected string
	}{
		{
			name:     "Add missing top-level key",
			dest:     "a: 1\n",
			tmpl:     "a: 1\nb: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "Add missing nested key",
			dest:     "a:\n  x: 1\n",
			tmpl:     "a:\n  x: 1\n  y: 2\n",
			expected: "a:\n    x: 1\n    y: 2\n",
		},
		{
			name:     "Preserve existing values",
			dest:     "a: 10\n",
			tmpl:     "a: 1\nb: 2\n",
			expected: "a: 10\nb: 2\n",
		},
		{
			name:     "Preserve existing nested values",
			dest:     "a:\n  x: 10\n",
			tmpl:     "a:\n  x: 1\n  y: 2\n",
			expected: "a:\n    x: 10\n    y: 2\n",
		},
		{
			name:     "No changes needed",
			dest:     "a: 1\nb: 2\n",
			tmpl:     "a: 1\nb: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "Dest is sequence, tmpl is mapping",
			dest:     "- 1\n- 2\n",
			tmpl:     "a: 1\n",
			expected: "- 1\n- 2\n",
		},
		{
			name:     "Dest is mapping, tmpl is sequence",
			dest:     "a: 1\n",
			tmpl:     "- 1\n- 2\n",
			expected: "a: 1\n",
		},
		{
			name:     "Key exists but dest is scalar and tmpl is mapping",
			dest:     "a: 10\n",
			tmpl:     "a:\n  x: 1\n",
			expected: "a: 10\n",
		},
		{
			name:     "Key exists but dest is mapping and tmpl is scalar",
			dest:     "a:\n  x: 1\n",
			tmpl:     "a: 10\n",
			expected: "a:\n    x: 1\n",
		},
		{
			name:     "Empty dest mapping",
			dest:     "{}\n",
			tmpl:     "a: 1\n",
			expected: "{a: 1}\n",
		},
		{
			name:     "Empty tmpl mapping",
			dest:     "a: 1\n",
			tmpl:     "{}\n",
			expected: "a: 1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var destNode, tmplNode yaml.Node
			err := yaml.Unmarshal([]byte(tt.dest), &destNode)
			if err != nil {
				t.Fatalf("Failed to unmarshal dest: %v", err)
			}
			err = yaml.Unmarshal([]byte(tt.tmpl), &tmplNode)
			if err != nil {
				t.Fatalf("Failed to unmarshal tmpl: %v", err)
			}

			if len(tmplNode.Content) > 0 && len(destNode.Content) > 0 {
				addMissingKeys(destNode.Content[0], tmplNode.Content[0])
			}

			outBytes, err := yaml.Marshal(&destNode)
			if err != nil {
				t.Fatalf("Failed to marshal result: %v", err)
			}
			outStr := string(outBytes)

			if outStr != tt.expected {
				t.Errorf("addMissingKeys() result:\n%s\nwant:\n%s", outStr, tt.expected)
			}
		})
	}
}

func TestFieldIsSet(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		path     []string
		expected bool
	}{
		{
			name:     "Top-level key set with value",
			content:  "a: hello\n",
			path:     []string{"a"},
			expected: true,
		},
		{
			name:     "Top-level key missing entirely",
			content:  "b: hello\n",
			path:     []string{"a"},
			expected: false,
		},
		{
			name:     "Top-level key present but empty string",
			content:  "a: \"\"\n",
			path:     []string{"a"},
			expected: false,
		},
		{
			name:     "Top-level key present but null",
			content:  "a: null\n",
			path:     []string{"a"},
			expected: false,
		},
		{
			name:     "Nested key set with value",
			content:  "connector:\n  group: frameflow\n",
			path:     []string{"connector", "group"},
			expected: true,
		},
		{
			name:     "Nested key missing, parent present",
			content:  "connector:\n  ipc_control_in: true\n",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Nested key missing, parent also missing",
			content:  "database:\n  dsn: /tmp/x.db\n",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Nested key present but empty string",
			content:  "connector:\n  group: \"\"\n",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Parent key exists but is a scalar, not a mapping",
			content:  "connector: not_a_map\n",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Empty document",
			content:  "",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Invalid YAML",
			content:  "connector:\n  - broken\n group: x",
			path:     []string{"connector", "group"},
			expected: false,
		},
		{
			name:     "Deeply nested key set with value",
			content:  "a:\n  b:\n    c: value\n",
			path:     []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "Deeply nested key missing at deepest level",
			content:  "a:\n  b:\n    x: value\n",
			path:     []string{"a", "b", "c"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fieldIsSet([]byte(tt.content), tt.path...)
			if result != tt.expected {
				t.Errorf("fieldIsSet() = %v, want %v", result, tt.expected)
			}
		})
	}
}
