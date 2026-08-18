package controlapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/db"
)

type templateNameReq struct {
	Name string `json:"name"`
}

func (s *Server) handleTemplatesList(w http.ResponseWriter, r *http.Request) {
	metas, err := db.ListTemplates(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"templates": metas})
}

// handleTemplateSave snapshots the current Z-layout and stores it under a name.
func (s *Server) handleTemplateSave(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeTemplateName(w, r)
	if !ok {
		return
	}
	yamlBytes, err := s.pm.SnapshotChromiumYAML()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := db.SaveTemplate(s.db, name, string(yamlBytes)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleTemplateApply materializes a stored template into the live settings file.
func (s *Server) handleTemplateApply(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeTemplateName(w, r)
	if !ok {
		return
	}
	yamlStr, err := db.GetTemplate(s.db, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if err := s.pm.ApplyChromiumTemplateYAML([]byte(yamlStr)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeTemplateName(w, r)
	if !ok {
		return
	}
	if err := db.DeleteTemplate(s.db, name); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// decodeTemplateName parses and validates the {name} body for POST endpoints.
func decodeTemplateName(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return "", false
	}
	var req templateNameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be 1-128 characters"})
		return "", false
	}
	return name, true
}
