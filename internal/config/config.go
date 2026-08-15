package config

import (
	"context"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"

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

	if oldConfig.Input.Resolution != newConfig.Input.Resolution {
		return DiffResult{RequiresRestart: true}
	}

	if outputsRequireRestart(oldConfig.Output, newConfig.Output) {
		return DiffResult{RequiresRestart: true}
	}

	if chromiumSourceDiff(oldConfig.Input.ChromiumSource, newConfig.Input.ChromiumSource) {
		return DiffResult{RequiresRestart: true}
	}

	return DiffResult{}
}

func chromiumSourceDiff(old, new models.ChromiumSource) bool {
	if old.Active != new.Active {
		return true
	}
	if old.Z0Active != new.Z0Active || old.Z0Path != new.Z0Path ||
		!ptrIntEqual(old.Z0Volume, new.Z0Volume) || !ptrIntEqual(old.Z0Width, new.Z0Width) || !ptrIntEqual(old.Z0Height, new.Z0Height) ||
		!ptrIntEqual(old.Z0X, new.Z0X) || !ptrIntEqual(old.Z0Y, new.Z0Y) {
		return true
	}
	if old.Z1Active != new.Z1Active || old.Z1Path != new.Z1Path ||
		!ptrIntEqual(old.Z1Volume, new.Z1Volume) || !ptrIntEqual(old.Z1Width, new.Z1Width) || !ptrIntEqual(old.Z1Height, new.Z1Height) ||
		!ptrIntEqual(old.Z1X, new.Z1X) || !ptrIntEqual(old.Z1Y, new.Z1Y) {
		return true
	}
	if old.Z2Active != new.Z2Active || old.Z2Path != new.Z2Path ||
		!ptrIntEqual(old.Z2Volume, new.Z2Volume) || !ptrIntEqual(old.Z2Width, new.Z2Width) || !ptrIntEqual(old.Z2Height, new.Z2Height) ||
		!ptrIntEqual(old.Z2X, new.Z2X) || !ptrIntEqual(old.Z2Y, new.Z2Y) {
		return true
	}
	if old.Z3Active != new.Z3Active || old.Z3Path != new.Z3Path ||
		!ptrIntEqual(old.Z3Volume, new.Z3Volume) || !ptrIntEqual(old.Z3Width, new.Z3Width) || !ptrIntEqual(old.Z3Height, new.Z3Height) ||
		!ptrIntEqual(old.Z3X, new.Z3X) || !ptrIntEqual(old.Z3Y, new.Z3Y) {
		return true
	}
	if old.Z4Active != new.Z4Active || old.Z4Path != new.Z4Path ||
		!ptrIntEqual(old.Z4Volume, new.Z4Volume) || !ptrIntEqual(old.Z4Width, new.Z4Width) || !ptrIntEqual(old.Z4Height, new.Z4Height) ||
		!ptrIntEqual(old.Z4X, new.Z4X) || !ptrIntEqual(old.Z4Y, new.Z4Y) {
		return true
	}
	if old.Z5Active != new.Z5Active || old.Z5Path != new.Z5Path ||
		!ptrIntEqual(old.Z5Volume, new.Z5Volume) || !ptrIntEqual(old.Z5Width, new.Z5Width) || !ptrIntEqual(old.Z5Height, new.Z5Height) ||
		!ptrIntEqual(old.Z5X, new.Z5X) || !ptrIntEqual(old.Z5Y, new.Z5Y) {
		return true
	}
	if old.Z6Active != new.Z6Active || old.Z6Path != new.Z6Path ||
		!ptrIntEqual(old.Z6Volume, new.Z6Volume) || !ptrIntEqual(old.Z6Width, new.Z6Width) || !ptrIntEqual(old.Z6Height, new.Z6Height) ||
		!ptrIntEqual(old.Z6X, new.Z6X) || !ptrIntEqual(old.Z6Y, new.Z6Y) {
		return true
	}
	if old.Z7Active != new.Z7Active || old.Z7Path != new.Z7Path ||
		!ptrIntEqual(old.Z7Volume, new.Z7Volume) || !ptrIntEqual(old.Z7Width, new.Z7Width) || !ptrIntEqual(old.Z7Height, new.Z7Height) ||
		!ptrIntEqual(old.Z7X, new.Z7X) || !ptrIntEqual(old.Z7Y, new.Z7Y) {
		return true
	}
	if old.Z8Active != new.Z8Active || old.Z8Path != new.Z8Path ||
		!ptrIntEqual(old.Z8Volume, new.Z8Volume) || !ptrIntEqual(old.Z8Width, new.Z8Width) || !ptrIntEqual(old.Z8Height, new.Z8Height) ||
		!ptrIntEqual(old.Z8X, new.Z8X) || !ptrIntEqual(old.Z8Y, new.Z8Y) {
		return true
	}
	return false
}

func ptrIntEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func outputsRequireRestart(old, new models.OutputSettings) bool {
	if old.Resolution != new.Resolution ||
		old.FPS != new.FPS ||
		old.VideoBitrate != new.VideoBitrate ||
		old.AudioBitrate != new.AudioBitrate {
		return true
	}

	return !slices.Equal(old.Destinations, new.Destinations)
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
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				// Add a small delay to ensure file is completely written, debounced
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(100 * time.Millisecond)
				timerC = timer.C
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				// File was replaced via atomic rename or removed, re-add the watch
				// Adding a short delay in a goroutine before re-adding to ensure the new file is in place
				go func(path string) {
					time.Sleep(50 * time.Millisecond)
					watcher.Add(path)
				}(w.path)

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
