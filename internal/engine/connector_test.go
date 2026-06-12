package engine

import (
	"os/exec"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestHandleControlCommand_Stream(t *testing.T) {
	pm := NewProcessManager(nil)
	pm.config = &models.Config{
		Output: models.OutputSettings{
			Active: false,
		},
	}

	// Enable stream
	cmdEnable := ControlCommand{
		Action: "set_input_state",
		Target: "stream",
		Payload: ControlPayload{
			Enabled: true,
		},
	}
	pm.handleControlCommand(cmdEnable)

	pm.mu.Lock()
	if !pm.config.Output.Active {
		t.Errorf("Expected config.Output.Active to be true")
	}
	pm.mu.Unlock()

	// Disable stream
	cmdMock := exec.Command("sleep", "10")
	if err := cmdMock.Start(); err != nil {
		t.Fatalf("Failed to start dummy command: %v", err)
	}
	pm.cmd = cmdMock

	cmdDisable := ControlCommand{
		Action: "set_input_state",
		Target: "stream",
		Payload: ControlPayload{
			Enabled: false,
		},
	}
	pm.handleControlCommand(cmdDisable)

	pm.mu.Lock()
	if pm.config.Output.Active {
		t.Errorf("Expected config.Output.Active to be false")
	}
	pm.mu.Unlock()

	err := cmdMock.Wait()
	if err == nil {
		t.Errorf("Expected command to be terminated, but it exited normally")
	}
}

func TestHandleControlCommand_Layer(t *testing.T) {
	pm := NewProcessManager(nil)
	pm.config = &models.Config{
		Input: models.InputSettings{
			FFmpegSource: models.FFmpegSource{
				Active: true,
				Layers: []models.Layer{
					{ID: 0, Active: false},
					{ID: 1, Active: true},
				},
			},
		},
	}

	// Enable layer 0
	cmdEnable := ControlCommand{
		Action: "set_input_state",
		Target: "layer0",
		Payload: ControlPayload{
			Enabled: true,
		},
	}
	pm.handleControlCommand(cmdEnable)

	pm.mu.Lock()
	if !pm.config.Input.FFmpegSource.Layers[0].Active {
		t.Errorf("Expected layer 0 to be active")
	}
	pm.mu.Unlock()

	// Disable layer 1
	cmdDisable := ControlCommand{
		Action: "set_input_state",
		Target: "layer1",
		Payload: ControlPayload{
			Enabled: false,
		},
	}
	pm.handleControlCommand(cmdDisable)

	pm.mu.Lock()
	if pm.config.Input.FFmpegSource.Layers[1].Active {
		t.Errorf("Expected layer 1 to be inactive")
	}
	pm.mu.Unlock()
}

func TestHandleControlCommand_LayerInvalidID(t *testing.T) {
	pm := NewProcessManager(nil)
	cmd := ControlCommand{
		Action: "set_input_state",
		Target: "layerABC",
		Payload: ControlPayload{
			Enabled: true,
		},
	}
	// Should not panic
	pm.handleControlCommand(cmd)
}

func TestHandleControlCommand_TriggerAlert(t *testing.T) {
	pm := NewProcessManager(nil)
	cmd := ControlCommand{
		Action:  "trigger_alert",
		Target:  "test",
		Payload: ControlPayload{Text: "alert"},
	}
	// Should not panic
	pm.handleControlCommand(cmd)
}

func TestHandleControlCommand_UnknownAction(t *testing.T) {
	pm := NewProcessManager(nil)
	cmd := ControlCommand{
		Action: "unknown_action",
	}
	// Should not panic
	pm.handleControlCommand(cmd)
}
