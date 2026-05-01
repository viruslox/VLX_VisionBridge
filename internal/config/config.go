package config

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/user/go-live-orchestrator/internal/models"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// LoadConfig parses the YAML configuration file.
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

type DiffResult struct {
	RequiresRestart      bool
	RequiresFilterUpdate bool
}

// DiffConfigs determines if a change requires a full FFmpeg restart or just a filter update.
func DiffConfigs(oldConfig, newConfig *models.Config) DiffResult {
	if oldConfig == nil || newConfig == nil {
		return DiffResult{RequiresRestart: true}
	}

	if outputsRequireRestart(oldConfig.Output, newConfig.Output) {
		return DiffResult{RequiresRestart: true}
	}

	return layersDiff(oldConfig.Layers, newConfig.Layers)
}

func outputsRequireRestart(old, new models.OutputSettings) bool {
	if old.Resolution != new.Resolution ||
		old.FPS != new.FPS ||
		old.VideoBitrate != new.VideoBitrate ||
		old.AudioBitrate != new.AudioBitrate {
		return true
	}

	if len(old.Destinations) != len(new.Destinations) {
		return true
	}
	for i := range old.Destinations {
		if old.Destinations[i] != new.Destinations[i] {
			return true
		}
	}

	return false
}

func layersDiff(old, new []models.Layer) DiffResult {
	var result DiffResult

	oldLayers := make(map[int]models.Layer)
	for _, l := range old {
		oldLayers[l.ID] = l
	}

	newLayers := make(map[int]models.Layer)
	for _, l := range new {
		newLayers[l.ID] = l
	}

	for id, newL := range newLayers {
		oldL, exists := oldLayers[id]
		if !exists {
			result.RequiresRestart = true // Adding a new layer conceptually needs restart here if inputs change
			continue
		}

		if oldL.InputType != newL.InputType || oldL.InputPath != newL.InputPath || oldL.Media != newL.Media {
			result.RequiresRestart = true
		} else if oldL.Active != newL.Active || oldL.Scale != newL.Scale || oldL.Crop != newL.Crop || oldL.Position != newL.Position {
			result.RequiresFilterUpdate = true
		}
	}

	for id := range oldLayers {
		if _, exists := newLayers[id]; !exists {
			result.RequiresRestart = true // Removing a layer conceptually needs restart
		}
	}

	return result
}

// Watcher handles watching the config file for changes
type Watcher struct {
	path      string
	onChange  func(*models.Config, DiffResult)
	current   *models.Config
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewWatcher(path string, onChange func(*models.Config, DiffResult)) *Watcher {
	return &Watcher{
		path:     path,
		onChange: onChange,
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cfg, err := LoadConfig(w.path)
	if err == nil {
		w.current = cfg
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(w.path)
	if err != nil {
		watcher.Close()
		return err
	}

	watchCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		defer watcher.Close()

		for {
			select {
			case <-watchCtx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// Add a small delay to ensure file is completely written
					time.Sleep(100 * time.Millisecond)

					newCfg, err := LoadConfig(w.path)
					if err != nil {
						log.Printf("Error reloading config: %v", err)
						continue
					}

					w.mu.Lock()
					diff := DiffConfigs(w.current, newCfg)
					w.current = newCfg
					w.mu.Unlock()

					if diff.RequiresRestart || diff.RequiresFilterUpdate {
						if w.onChange != nil {
							w.onChange(newCfg, diff)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	return nil
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	w.mu.Unlock()
	w.wg.Wait()
}
