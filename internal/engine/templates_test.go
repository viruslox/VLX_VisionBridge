package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotAndApplyChromiumTemplateYAML(t *testing.T) {
	// 1. Create a temporary config file with full structure
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "visionbridge.settings")
	err := os.Setenv("CONFIG_PATH", configPath)
	require.NoError(t, err)
	defer os.Unsetenv("CONFIG_PATH")

	initialConfig := `
output:
  active: true
  resolution: "1920x1080"
input:
  chromium_source:
    z0_active: true
    z0_path: "initial_path"
    z1_active: false
`
	err = os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	pm := NewProcessManager(nil)

	// 2. Snapshot the initial layout
	snapshot, err := pm.SnapshotChromiumYAML()
	require.NoError(t, err)
	require.Contains(t, string(snapshot), "chromium_source:")
	require.Contains(t, string(snapshot), "z0_path: \"initial_path\"")

	// 3. Mutate the live config file (both layout and other fields)
	mutatedConfig := `
output:
  active: false
  resolution: "1280x720"
input:
  chromium_source:
    z0_active: false
    z0_path: "mutated_path"
    z1_active: true
`
	err = os.WriteFile(configPath, []byte(mutatedConfig), 0644)
	require.NoError(t, err)

	// 4. Apply the snapshot back to the live config
	err = pm.ApplyChromiumTemplateYAML(snapshot)
	require.NoError(t, err)

	// 5. Verify the chromium_source is restored to the snapshot, but other fields are left untouched
	finalConfigBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)
	finalConfigStr := string(finalConfigBytes)

	// Output fields should remain mutated
	assert.Contains(t, finalConfigStr, "active: false")
	assert.Contains(t, finalConfigStr, "resolution: \"1280x720\"")

	// Chromium source should be restored
	assert.Contains(t, finalConfigStr, "z0_active: true")
	assert.Contains(t, finalConfigStr, "z0_path: \"initial_path\"")
	assert.Contains(t, finalConfigStr, "z1_active: false")
}
