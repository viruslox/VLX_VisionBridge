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

	for _, newL := range new {
		var found bool
		for _, oldL := range old {
			if oldL.ID == newL.ID {
				found = true
				if oldL.InputType != newL.InputType || oldL.InputPath != newL.InputPath || oldL.Media != newL.Media {
					result.RequiresRestart = true
				} else if oldL.Active != newL.Active || oldL.Scale != newL.Scale || oldL.Crop != newL.Crop || oldL.Position != newL.Position {
					result.RequiresFilterUpdate = true
				}
				break
			}
		}
		if !found {
			result.RequiresRestart = true
		}
	}

	for _, oldL := range old {
		var found bool
		for _, newL := range new {
			if newL.ID == oldL.ID {
				found = true
				break
			}
		}
		if !found {
			result.RequiresRestart = true
		}
	}

	return result
}

// Watcher handles watching the config file for changes
type Watcher struct {
	path     string
	onChange func(*models.Config, DiffResult)
	current  *models.Config
	mu       sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
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

	go w.watchEvents(watchCtx, watcher)

	return nil
}

func (w *Watcher) watchEvents(ctx context.Context, watcher *fsnotify.Watcher) {
	defer w.wg.Done()
	defer watcher.Close()

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Add a small delay to ensure file is completely written, debounced
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(100 * time.Millisecond)
				timerC = timer.C
			}
		case <-timerC:
			timer = nil
			timerC = nil

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
		case err, ok := <-watcher.Errors:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
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
