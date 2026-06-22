package engine

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
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
	// Avvia il server HTTP / WebSocket locale per Chromium DOM
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println("WS upgrade failed:", err)
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
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		})

		mux.HandleFunc("/api/list-dir", func(w http.ResponseWriter, r *http.Request) {
			dirPath := r.URL.Query().Get("path")
			if dirPath == "" {
				http.Error(w, "missing path", http.StatusBadRequest)
				return
			}
			files, err := os.ReadDir(dirPath)
			if err != nil {
				http.Error(w, "read dir error", http.StatusInternalServerError)
				return
			}

			var mediaFiles []string
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(f.Name()))
				if ext == ".mp4" || ext == ".webm" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".mp3" {
					mediaFiles = append(mediaFiles, f.Name())
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mediaFiles)
		})

		log.Println("Starting local HTTP/WS server on 127.0.0.1:50001")
		if err := http.ListenAndServe("127.0.0.1:50001", mux); err != nil {
			log.Printf("Local HTTP/WS server failed: %v", err)
		}
	}()

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
		if strings.HasPrefix(cmd.Target, "overlay@layer") {
			idStr := strings.TrimPrefix(cmd.Target, "overlay@layer")
			zLayer := "z" + idStr
			wsCmd := map[string]interface{}{
				"layer": zLayer,
			}
			if cmd.Payload.Enabled {
				wsCmd["action"] = "play"
				wsCmd["path"] = cmd.Payload.Text
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
		log.Printf("trigger_alert action received for target %s, text: %s", cmd.Target, cmd.Payload.Text)
		// Custom logic can be handled here or via ZMQ if drawtext is implemented.
	} else if cmd.Action == "reload" && cmd.Target == "chromium" {
		log.Println("Reloading Chromium overlay via control command")
		pm.ReloadChromium()
	} else {
		log.Printf("Unknown action received: %s", cmd.Action)
	}
}

func (pm *ProcessManager) broadcastWSMessage(msg interface{}) {
	pm.wsMutex.Lock()
	defer pm.wsMutex.Unlock()
	for client := range pm.wsClients {
		if err := client.WriteJSON(msg); err != nil {
			client.Close()
			delete(pm.wsClients, client)
		}
	}
}
