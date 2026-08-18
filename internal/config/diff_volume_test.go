package config

import (
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestDiffConfigs_VolumeOnly(t *testing.T) {
	oldVol := 100
	newVol := 50
	oldCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: true,
				Z1Volume: &oldVol,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}
	newCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: true,
				Z1Volume: &newVol,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}

	diff := DiffConfigs(oldCfg, newCfg)
	if diff.RequiresRestart {
		t.Errorf("Expected requiresRestart=false for volume only change, got %v", diff.RequiresRestart)
	}
	if !diff.RequiresFilterUpdate {
		t.Errorf("Expected requiresFilterUpdate=true for volume only change, got %v", diff.RequiresFilterUpdate)
	}
}

func TestDiffConfigs_ChromiumSourceActive(t *testing.T) {
	oldCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: true,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}
	newCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: false,
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}

	diff := DiffConfigs(oldCfg, newCfg)
	if !diff.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for chromium_source active change, got %v", diff.RequiresRestart)
	}
}

func TestDiffConfigs_Z12Path(t *testing.T) {
	oldCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: true,
				Z12Path: "/old/path",
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}
	newCfg := &models.Config{
		Input: models.InputSettings{
			ChromiumSource: models.ChromiumSource{
				Active: true,
				Z12Path: "/new/path",
			},
		},
		Output: models.OutputSettings{Resolution: "1920x1080"},
	}

	diff := DiffConfigs(oldCfg, newCfg)
	if !diff.RequiresRestart {
		t.Errorf("Expected requiresRestart=true for Z12Path change, got %v", diff.RequiresRestart)
	}
}
