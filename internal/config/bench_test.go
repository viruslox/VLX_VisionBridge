package config

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/go-live-orchestrator/internal/models"
)

func BenchmarkWatcherRapidWrites(b *testing.B) {
	yamlContent1 := `
output:
  resolution: "1920x1080"
  fps: 60
layers:
  - id: 1
    active: true
    input_type: "folder"
    input_path: "/path1"
`
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(yamlContent1), 0644)
	if err != nil {
		b.Fatalf("Failed to write temp config file: %v", err)
	}

	var callbackCount int32
	watcher := NewWatcher(configFile, func(cfg *models.Config, diff DiffResult) {
		atomic.AddInt32(&callbackCount, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Send multiple events to the watcher directly since os.WriteFile
		// + fsnotify might batch them automatically or ignore due to timestamp granularity
		for j := 0; j < 5; j++ {
			os.WriteFile(configFile, []byte(yamlContent1), 0644)
		}
		// The core of the issue is that we process sequentially with a blocking Sleep.
		// Wait enough to allow unoptimized to finish
		time.Sleep(100 * time.Millisecond)
	}
}
