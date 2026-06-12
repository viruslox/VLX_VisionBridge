package engine

import (
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestStartConnectorListener(t *testing.T) {
	// Initialize the ProcessManager
	pm := NewProcessManager(nil)
	pm.config = &models.Config{
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
