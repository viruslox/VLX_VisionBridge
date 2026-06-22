package engine

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (pm *ProcessManager) StartConnectorListener() {
	// Start local HTTP / WebSocket server for Chromium DOM control plane
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println("WebSocket upgrade protocol failed:", err)
				return
			}
			pm.wsMutex.Lock()
			if pm.wsClients == nil {
				pm.wsClients = make(map[*websocket.Conn]bool)
			}
			pm.wsClients[conn] = true
			pm.wsMutex.Unlock()

			defer func() {
				pm.wsMutex.Lock()
				delete(pm.wsClients, conn)
				pm.wsMutex.Unlock()
				conn.Close()
			}()

			for {
				if _, msg, err := conn.ReadMessage(); err != nil {
					break
				} else {
					var req map[string]string
					if json.Unmarshal(msg, &req) == nil && req["action"] == "hello" {
						// Synchronize DOM state upon initial client connection
						pm.mu.Lock()
						cfg := pm.config
						pm.mu.Unlock()
						if cfg != nil {
							syncMsg := pm.buildSyncMessage(cfg)
							_ = conn.WriteJSON(syncMsg)
						}
					}
				}
			}
		})

		log.Println("Initializing local HTTP/WS control server on 127.0.0.1:50001")
		if err := http.ListenAndServe("127.0.0.1:50001", mux); err != nil {
			log.Printf("Local HTTP/WS server initialization failed: %v", err)
		}
	}()

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

				pm.handleControlCommand(cmd)
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
			wsCmd := map[string]interface{}{
				"layer": zLayer,
			}
			if cmd.Payload.Enabled {
				wsCmd["action"] = "play"
				wsCmd["files"] = pm.ResolvePath(cmd.Payload.Text)
			} else {
				wsCmd["action"] = "hide"
			}
			pm.broadcastWSMessage(wsCmd)
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
