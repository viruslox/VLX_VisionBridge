package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var configMutex sync.Mutex

func resolveConfigPath() string {
	envPath := os.Getenv("CONFIG_PATH")
	if envPath != "" {
		return envPath
	}
	if _, err := os.Stat("configs/visionbridge.settings"); err == nil {
		return "configs/visionbridge.settings"
	}
	return "/opt/VLX_VisionBridge/etc/visionbridge.settings"
}

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
		log.Println("IPC Control Inbound directive is disabled in configuration. Bypassing connector listener.")
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

	// Purge stale socket descriptors to prevent binding collisions
	if _, err := os.Stat(sockPath); err == nil {
		if err := os.Remove(sockPath); err != nil {
			log.Printf("Failed to purge stale vlx_control socket at %s: %v", sockPath, err)
		}
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Printf("Failed to bind vlx_control IPC listener: %v", err)
		return
	}
	defer listener.Close()

	if err := os.Chmod(sockPath, 0770); err != nil {
		log.Printf("Warning: Unable to assign permissions on socket descriptor %s: %v", sockPath, err)
	}

	var uid, gid int = -1, -1

	u, err := user.Lookup("visionbridge")
	if err != nil {
		log.Printf("Warning: System user 'visionbridge' unresolved, bypassing UID ownership assignment.")
	} else {
		if parsedUID, err := strconv.Atoi(u.Uid); err == nil {
			uid = parsedUID
		}
	}

	g, err := user.LookupGroup(groupName)
	if err != nil {
		log.Printf("Warning: System group '%s' unresolved, bypassing GID ownership assignment.", groupName)
	} else {
		if parsedGID, err := strconv.Atoi(g.Gid); err == nil {
			gid = parsedGID
		}
	}

	if uid != -1 || gid != -1 {
		if err := os.Chown(sockPath, uid, gid); err != nil {
			log.Printf("Warning: Failed to execute ownership assignment on socket %s: %v", sockPath, err)
		} else {
			log.Printf("Socket %s ownership explicitly bound to UID %d, GID %d", sockPath, uid, gid)
		}
	}

	log.Printf("IPC socket active. Listening for control directives on %s", sockPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept inbound vlx_control connection stream: %v", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			decoder := json.NewDecoder(c)
			for {
				var cmd ControlCommand
				if err := decoder.Decode(&cmd); err != nil {
					if err.Error() != "EOF" {
						log.Printf("JSON payload decoding failure from vlx_control IPC: %v", err)
					}
					break
				}

				log.Printf("Received control command via IPC: %+v", cmd)

				pm.handleControlCommand(cmd)

				log.Printf("Execution of control command '%s' for target '%s' completed successfully.", cmd.Action, cmd.Target)
			}
		}(conn)
	}
}

func (pm *ProcessManager) handleControlCommand(cmd ControlCommand) {
	if cmd.Action == "set_input_state" && cmd.Target == "stream" {
		pm.mu.Lock()
		log.Printf("Stream output runtime active state requested to transition: %v", cmd.Payload.Enabled)
		if pm.config != nil {
			pm.config.Output.Active = cmd.Payload.Enabled
		}

		// Hard termination of FFmpeg process to immediately halt stream broadcast
		if !cmd.Payload.Enabled && pm.cmd != nil && pm.cmd.Process != nil {
			log.Println("Executing SIGKILL on active FFmpeg process to terminate stream transmission...")
			pm.cmd.Process.Kill()
		}
		pm.mu.Unlock()
		return
	}

	if cmd.Action == "set_input_state" {
		if strings.HasPrefix(cmd.Target, "overlay@layer") {
			idStr := strings.TrimPrefix(cmd.Target, "overlay@layer")
			zLayer := "z" + idStr

			configMutex.Lock()

			configPath := resolveConfigPath()
			data, err := os.ReadFile(configPath)
			if err != nil {
				log.Printf("Failed to read configuration file: %v", err)
				configMutex.Unlock()
				return
			}

			var node yaml.Node
			if err := yaml.Unmarshal(data, &node); err != nil {
				log.Printf("Failed to parse configuration file: %v", err)
				configMutex.Unlock()
				return
			}

			if len(node.Content) > 0 {
				doc := node.Content[0]
				for i := 0; i < len(doc.Content); i += 2 {
					if doc.Content[i].Value == "input" {
						inputMap := doc.Content[i+1]
						for j := 0; j < len(inputMap.Content); j += 2 {
							if inputMap.Content[j].Value == "chromium_source" {
								chromMap := inputMap.Content[j+1]
								activeKey := fmt.Sprintf("%s_active", zLayer)
								pathKey := fmt.Sprintf("%s_path", zLayer)
								for k := 0; k < len(chromMap.Content); k += 2 {
									if chromMap.Content[k].Value == activeKey {
										if cmd.Payload.Enabled {
											chromMap.Content[k+1].Value = "true"
										} else {
											chromMap.Content[k+1].Value = "false"
										}
									}
									if cmd.Payload.Enabled && chromMap.Content[k].Value == pathKey {
										resolvedPaths := pm.ResolvePath(cmd.Payload.Text)
										if len(resolvedPaths) > 0 {
											chromMap.Content[k+1].Value = resolvedPaths[0]
										} else {
											chromMap.Content[k+1].Value = cmd.Payload.Text
										}
									}
								}
							}
						}
					}
				}
			}

			out, err := yaml.Marshal(&node)
			if err != nil {
				log.Printf("Failed to marshal updated configuration: %v", err)
				configMutex.Unlock()
				return
			}

			tmpFile := configPath + ".tmp"
			if err := os.WriteFile(tmpFile, out, 0644); err != nil {
				log.Printf("Failed to write temporary configuration file: %v", err)
				configMutex.Unlock()
				return
			}
			if err := os.Rename(tmpFile, configPath); err != nil {
				os.Remove(tmpFile)
				log.Printf("Failed to rename temporary configuration file: %v", err)
				configMutex.Unlock()
				return
			}

			configMutex.Unlock()
			log.Printf("Updated configuration for %s (Enabled: %v)", zLayer, cmd.Payload.Enabled)

		} else if strings.HasPrefix(cmd.Target, "volume@layer") {
			idStr := strings.TrimPrefix(cmd.Target, "volume@layer")
			zLayer := "z" + idStr
			vol, _ := strconv.Atoi(cmd.Payload.Text)
			wsCmd := map[string]interface{}{
				"layer":  zLayer,
				"action": "volume",
				"volume": vol,
			}
			pm.broadcastWSMessage(wsCmd)
		}
	} else if cmd.Action == "trigger_alert" {
		log.Printf("trigger_alert event directive received for target %s, text payload: %s", cmd.Target, cmd.Payload.Text)
	} else if cmd.Action == "reload" && cmd.Target == "chromium" {
		log.Println("Executing manual restart of Chromium DOM rendering engine via IPC directive")
		pm.ReloadChromium()
	} else {
		log.Printf("Unrecognized control action directive received: %s", cmd.Action)
	}
}
