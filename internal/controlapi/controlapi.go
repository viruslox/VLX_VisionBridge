// Package controlapi exposes the always-on control and status HTTP API used by
// the VLX_VisionBridge web GUI. It runs independently of the GStreamer pipeline
// and the IPC connector, and reuses the engine's proven control handler so
// toggles take the same atomic-config-edit / live-apply paths as IPC commands.
package controlapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/user/VLX_VisionBridge/internal/engine"
	"github.com/user/VLX_VisionBridge/internal/models"
)

// layerCount is the number of Chromium Z-layers (Z0..Z12).
const layerCount = 13

// Server is the control/status HTTP API.
type Server struct {
	pm       *engine.ProcessManager
	user     string
	pass     string
	shutdown func()
	httpSrv  *http.Server
}

// New builds the control API server. If user is empty, requests are not
// authenticated (the 127.0.0.1 bind is then the only trust boundary).
func New(pm *engine.ProcessManager, bindAddr, port, user, pass string, shutdown func()) *Server {
	s := &Server{pm: pm, user: user, pass: pass, shutdown: shutdown}

	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	if port == "" {
		port = "8770"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.auth(s.handleStatus))
	mux.HandleFunc("/api/output", s.auth(s.handleOutput))
	mux.HandleFunc("/api/layer", s.auth(s.handleLayer))
	mux.HandleFunc("/api/volume", s.auth(s.handleVolume))
	mux.HandleFunc("/api/shutdown", s.auth(s.handleShutdown))

	s.httpSrv = &http.Server{Addr: bindAddr + ":" + port, Handler: mux}
	return s
}

// Start runs the HTTP server in the background.
func (s *Server) Start() {
	go func() {
		log.Printf("[ControlAPI] listening on %s", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ControlAPI] server error: %v", err)
		}
	}()
}

// Stop gracefully shuts the HTTP server down.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.httpSrv.Shutdown(ctx)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.user != "" {
			u, p, ok := r.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(u), []byte(s.user)) != 1 ||
				subtle.ConstantTimeCompare([]byte(p), []byte(s.pass)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="VLX_VisionBridge"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type layerStatus struct {
	Index  int    `json:"index"`
	Active bool   `json:"active"`
	Path   string `json:"path"`
	Volume int    `json:"volume"`
}

type statusResponse struct {
	Output struct {
		Active     bool   `json:"active"`
		Resolution string `json:"resolution"`
		FPS        int    `json:"fps"`
	} `json:"output"`
	Chromium struct {
		Active bool `json:"active"`
	} `json:"chromium"`
	OverlayServerActive bool          `json:"overlay_server_active"`
	Resolution          string        `json:"resolution"`
	Framerate           int           `json:"framerate"`
	Layers              []layerStatus `json:"layers"`
}

// layerState reads a Z-layer's fields by index from the flat ChromiumSource
// struct (Z0..Z12) via reflection, so the 13 layers don't need 13 hand-written
// switch arms. Volume is a *int; nil is reported as 100.
func layerState(c models.ChromiumSource, i int) (active bool, path string, volume int) {
	v := reflect.ValueOf(c)
	if af := v.FieldByName(fmt.Sprintf("Z%dActive", i)); af.IsValid() {
		active = af.Bool()
	}
	if pf := v.FieldByName(fmt.Sprintf("Z%dPath", i)); pf.IsValid() {
		path = pf.String()
	}
	volume = 100
	if vf := v.FieldByName(fmt.Sprintf("Z%dVolume", i)); vf.IsValid() && !vf.IsNil() {
		volume = int(vf.Elem().Int())
	}
	return active, path, volume
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	cfg := s.pm.ConfigSnapshot()

	var resp statusResponse
	resp.Output.Active = cfg.Output.Active
	resp.Output.Resolution = cfg.Output.Resolution
	resp.Output.FPS = cfg.Output.FPS
	resp.Chromium.Active = cfg.Input.ChromiumSource.Active
	resp.OverlayServerActive = cfg.Input.OverlayServerActive
	resp.Resolution = cfg.Input.Resolution
	resp.Framerate = cfg.Input.Framerate

	for i := 0; i < layerCount; i++ {
		active, path, volume := layerState(cfg.Input.ChromiumSource, i)
		resp.Layers = append(resp.Layers, layerStatus{Index: i, Active: active, Path: path, Volume: volume})
	}

	writeJSON(w, http.StatusOK, resp)
}

type outputReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req outputReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	s.pm.Dispatch(engine.ControlCommand{
		Action:  "set_input_state",
		Target:  "stream",
		Payload: engine.ControlPayload{Enabled: req.Enabled},
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type layerReq struct {
	Index   int     `json:"index"`
	Enabled bool    `json:"enabled"`
	Path    *string `json:"path"` // optional; when enabling and omitted, current path is preserved
}

func (s *Server) handleLayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req layerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Index < 0 || req.Index >= layerCount {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "layer index out of range (0-12)"})
		return
	}

	// The engine writes zN_path whenever a layer is enabled. To avoid blanking an
	// existing path on a pure active-toggle, default the text to the current path
	// unless the caller explicitly supplies one.
	text := ""
	if req.Enabled {
		if req.Path != nil {
			text = *req.Path
		} else {
			cfg := s.pm.ConfigSnapshot()
			_, cur, _ := layerState(cfg.Input.ChromiumSource, req.Index)
			text = cur
		}
	}

	s.pm.Dispatch(engine.ControlCommand{
		Action:  "set_input_state",
		Target:  fmt.Sprintf("overlay@layer%d", req.Index),
		Payload: engine.ControlPayload{Enabled: req.Enabled, Text: text},
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type volumeReq struct {
	Index  int `json:"index"`
	Volume int `json:"volume"`
}

func (s *Server) handleVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req volumeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Index < 0 || req.Index >= layerCount {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "layer index out of range (0-12)"})
		return
	}
	s.pm.Dispatch(engine.ControlCommand{
		Action:  "set_input_state",
		Target:  fmt.Sprintf("volume@layer%d", req.Index),
		Payload: engine.ControlPayload{Text: strconv.Itoa(req.Volume)},
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go func() {
		time.Sleep(150 * time.Millisecond)
		if s.shutdown != nil {
			s.shutdown()
		}
	}()
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
