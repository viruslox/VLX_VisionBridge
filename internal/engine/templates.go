package engine

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SnapshotChromiumYAML reads the live settings file and returns the current
// chromium_source (Z-layout) block as a standalone YAML fragment
// ("chromium_source: ...") suitable for storage and later re-application.
func (pm *ProcessManager) SnapshotChromiumYAML() ([]byte, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	configPath := resolveConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	chrom := findChromiumSourceNode(&node)
	if chrom == nil {
		return nil, fmt.Errorf("live config has no input.chromium_source block")
	}

	wrapper := yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "chromium_source"},
			chrom,
		},
	}
	out, err := yaml.Marshal(&wrapper)
	if err != nil {
		return nil, fmt.Errorf("marshal chromium_source: %w", err)
	}
	return out, nil
}

// ApplyChromiumTemplateYAML splices the chromium_source block from the given
// YAML fragment into the live settings file (atomic temp + rename), so the
// config watcher hot-reloads the new layout -- the same materialize path used
// by file-based templates.
func (pm *ProcessManager) ApplyChromiumTemplateYAML(templateData []byte) error {
	var templateNode yaml.Node
	if err := yaml.Unmarshal(templateData, &templateNode); err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	templateChrom := findChromiumSourceNode(&templateNode)
	if templateChrom == nil {
		return fmt.Errorf("template has no chromium_source block")
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	configPath := resolveConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	liveChrom := findChromiumSourceNode(&node)
	if liveChrom == nil {
		return fmt.Errorf("live config has no input.chromium_source block")
	}
	liveChrom.Content = templateChrom.Content

	out, err := yaml.Marshal(&node)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmpFile := configPath + ".tmp"
	if err := os.WriteFile(tmpFile, out, 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpFile, configPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp config: %w", err)
	}
	return nil
}
