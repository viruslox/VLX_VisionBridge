package engine

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
)

type ControlPayload struct {
	Enabled bool   `json:"enabled"`
	Text    string `json:"text"`
}

type ControlCommand struct {
	EventID   string         `json:"event_id"`
	Timestamp int64          `json:"timestamp"`
	Action    string         `json:"action"`
	Target    string         `json:"target"`
	Payload   ControlPayload `json:"payload"`
}

func (pm *ProcessManager) StartConnectorListener() {
	if pm.config != nil && !pm.config.Connector.IPCControlIn {
		log.Println("IPC Control In is disabled in configuration. Skipping connector listener.")
		return
	}

	sockPath := "/tmp/vlx_control.sock"
	groupName := "frameflow"
	if pm.config != nil && pm.config.Connector.ControlSocket != "" {
		sockPath = pm.config.Connector.ControlSocket
	}
	if pm.config != nil && pm.config.Connector.Group != "" {
		groupName = pm.config.Connector.Group
	}

	// Cleanup mechanism: Ensure socket file is removed before binding
	// to prevent "address already in use" errors if the app crashed previously.
	if _, err := os.Stat(sockPath); err == nil {
		if err := os.Remove(sockPath); err != nil {
			log.Printf("Failed to remove existing vlx_control socket at %s: %v", sockPath, err)
		}
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Printf("Failed to start vlx_control listener: %v", err)
		return
	}
	defer listener.Close()

	// Apply permissions to the socket
	if err := os.Chmod(sockPath, 0770); err != nil {
		log.Printf("Warning: Failed to set permissions on socket %s: %v", sockPath, err)
	}

	// Change ownership
	var uid, gid int = -1, -1

	// Lookup user visionbridge
	u, err := user.Lookup("visionbridge")
	if err != nil {
		log.Printf("Warning: User 'visionbridge' not found, skipping user ownership change for socket.")
	} else {
		if parsedUID, err := strconv.Atoi(u.Uid); err == nil {
			uid = parsedUID
		}
	}

	// Lookup group
	g, err := user.LookupGroup(groupName)
	if err != nil {
		log.Printf("Warning: Group '%s' not found, skipping group ownership change for socket.", groupName)
	} else {
		if parsedGID, err := strconv.Atoi(g.Gid); err == nil {
			gid = parsedGID
		}
	}

	if uid != -1 || gid != -1 {
		if err := os.Chown(sockPath, uid, gid); err != nil {
			log.Printf("Warning: Failed to set ownership on socket %s: %v", sockPath, err)
		} else {
			log.Printf("Socket %s ownership set to UID %d, GID %d", sockPath, uid, gid)
		}
	}

	log.Printf("Listening for control commands on %s", sockPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept vlx_control connection: %v", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			decoder := json.NewDecoder(c)
			for {
				var cmd ControlCommand
				if err := decoder.Decode(&cmd); err != nil {
					if err.Error() != "EOF" {
						log.Printf("Failed to decode JSON from vlx_control: %v", err)
					}
					break
				}

				pm.handleControlCommand(cmd)
			}
		}(conn)
	}
}

func (pm *ProcessManager) handleControlCommand(cmd ControlCommand) {
	if cmd.Action == "set_input_state" && cmd.Target == "stream" {
		pm.mu.Lock()
		log.Printf("Stream output Active state changing to: %v", cmd.Payload.Enabled)
		if pm.config != nil {
			pm.config.Output.Active = cmd.Payload.Enabled
		}

		// If turning OFF the stream, brutally kill the running FFmpeg process
		if !cmd.Payload.Enabled && pm.cmd != nil && pm.cmd.Process != nil {
			log.Println("Killing active FFmpeg process to stop stream...")
			pm.cmd.Process.Kill()
		}
		pm.mu.Unlock()
		return
	}

	if cmd.Action == "set_input_state" {
		if strings.HasPrefix(cmd.Target, "layer") {
			idStr := strings.TrimPrefix(cmd.Target, "layer")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				log.Printf("Invalid target layer ID in control command: %s", cmd.Target)
				return
			}

			pm.mu.Lock() // UPDATE STATE IN MEMORY WITHOUT RESTARTING FFMPEG
			if pm.config != nil && pm.config.Input.FFmpegSource.Active {
				for i, layer := range pm.config.Input.FFmpegSource.Layers {
					if layer.ID == id {
						if pm.config.Input.FFmpegSource.Layers[i].Active != cmd.Payload.Enabled {
							log.Printf("Setting layer %d Active to %v via control command", id, cmd.Payload.Enabled)
							pm.config.Input.FFmpegSource.Layers[i].Active = cmd.Payload.Enabled

							// Copy config
							newCfg := *pm.config
							pm.mu.Unlock()

							pm.mu.Lock() // UPDATE STATE IN MEMORY WITHOUT RESTARTING FFMPEG
							pm.config = &newCfg
							pm.mu.Unlock()

							pm.UpdateFilter(&newCfg)
							return
						}
						break
					}
				}
			}
			pm.mu.Unlock()
		}
	} else if cmd.Action == "trigger_alert" {
		log.Printf("trigger_alert action received for target %s, text: %s", cmd.Target, cmd.Payload.Text)
		// Custom logic can be handled here or via ZMQ if drawtext is implemented.
	} else if cmd.Action == "reload" && cmd.Target == "chromium" {
		log.Println("Reloading Chromium overlay via control command")
		pm.ReloadChromium()
	} else {
		log.Printf("Unknown action received: %s", cmd.Action)
	}
}
