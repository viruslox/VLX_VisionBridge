package engine

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestStartConnectorListener(t *testing.T) {
	// Initialize the ProcessManager
	pm := NewProcessManager(nil)
	pm.config = &models.Config{
		Connector: models.ConnectorSettings{
			IPCControlIn:  true,
			Group:         "frameflow",
			ControlSocket: "/tmp/vlx_control.sock",
		},
		Output: models.OutputSettings{
			Active: false,
		},
	}

	sockPath := "/tmp/vlx_control.sock"

	// 1. Test Cleanup Mechanism
	// Create a dummy file to ensure it gets cleaned up before binding
	file, err := os.Create(sockPath)
	if err == nil {
		file.Close()
	} else {
		t.Fatalf("Failed to create dummy socket file: %v", err)
	}

	// 2. Start the connector listener in the background
	go pm.StartConnectorListener()

	// Wait for the socket to be available
	var conn net.Conn
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		conn, err = net.Dial("unix", sockPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to connect to vlx_control socket: %v", err)
	}
	defer conn.Close()
	defer os.Remove(sockPath)

	// 3. Send a valid ControlCommand JSON payload
	cmd := ControlCommand{
		EventID:   "test-123",
		Timestamp: time.Now().Unix(),
		Action:    "set_input_state",
		Target:    "stream",
		Payload: ControlPayload{
			Enabled: true,
		},
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(&cmd); err != nil {
		t.Fatalf("Failed to encode and send JSON payload: %v", err)
	}

	// 4. Wait a moment for the command to be processed
	time.Sleep(100 * time.Millisecond)

	// 5. Verify the configuration state changed
	pm.mu.Lock()
	active := pm.config.Output.Active
	pm.mu.Unlock()

	if !active {
		t.Errorf("Expected Output.Active to be true, got %v", active)
	}
}

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
