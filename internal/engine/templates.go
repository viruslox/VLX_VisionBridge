package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TemplateFileName sanitizes a template name into a safe .yaml filename inside
// the VisionBridge configuration directory, rejecting path traversal or
// reserved settings filenames.
func TemplateFileName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("invalid template name %q", name)
	}
	base := filepath.Base(trimmed)
	if base == "" || base == "." || base == ".." {
		return "", fmt.Errorf("invalid template name %q", name)
	}
	lower := strings.ToLower(base)
	if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
		base += ".yaml"
		lower = strings.ToLower(base)
	}
	switch lower {
	case "visionbridge.settings", "frontend.settings", "mediamtx_visionbridge.settings":
		return "", fmt.Errorf("reserved configuration filename %q", base)
	}
	return base, nil
}

// HasTemplateFile reports whether a template already has a corresponding .yaml
// file in the VisionBridge configuration directory alongside visionbridge.settings,
// returning the resolved filename.
func (pm *ProcessManager) HasTemplateFile(name string) (bool, string) {
	fileName, err := TemplateFileName(name)
	if err != nil {
		return false, ""
	}
	configDir := filepath.Dir(resolveConfigPath())
	fullPath := filepath.Join(configDir, fileName)
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		return true, fileName
	}
	// Also check exact base name if the caller stored a custom suffix.
	rawBase := filepath.Base(strings.TrimSpace(name))
	if rawBase != fileName && rawBase != "." && rawBase != ".." {
		if info, err := os.Stat(filepath.Join(configDir, rawBase)); err == nil && !info.IsDir() {
			return true, rawBase
		}
	}
	return false, fileName
}

// SaveTemplateToFile writes a template's chromium_source YAML fragment to a
// .yaml file in the VisionBridge configuration directory (atomic temp + rename).
func (pm *ProcessManager) SaveTemplateToFile(name string, templateData []byte) (string, error) {
	fileName, err := TemplateFileName(name)
	if err != nil {
		return "", err
	}

	var templateNode yaml.Node
	if err := yaml.Unmarshal(templateData, &templateNode); err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	if findChromiumSourceNode(&templateNode) == nil {
		return "", fmt.Errorf("template has no chromium_source block")
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	configDir := filepath.Dir(resolveConfigPath())
	targetPath := filepath.Join(configDir, fileName)
	tmpFile := targetPath + ".tmp"
	if err := os.WriteFile(tmpFile, templateData, 0644); err != nil {
		return "", fmt.Errorf("write temp template file: %w", err)
	}
	if err := os.Rename(tmpFile, targetPath); err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("rename temp template file: %w", err)
	}
	return fileName, nil
}

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
