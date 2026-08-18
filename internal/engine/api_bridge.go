package engine

import "github.com/user/VLX_VisionBridge/internal/models"

// Dispatch executes a control command through the same handler used by the IPC
// connector, so the HTTP control API reuses the proven, atomic config-editing
// and live-apply paths rather than duplicating them.
func (pm *ProcessManager) Dispatch(cmd ControlCommand) {
	pm.handleControlCommand(cmd)
}

// ConfigSnapshot returns a copy of the engine's current in-memory configuration
// for read-only status reporting. It is taken under the manager lock.
func (pm *ProcessManager) ConfigSnapshot() models.Config {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.config == nil {
		return models.Config{}
	}
	return *pm.config
}
