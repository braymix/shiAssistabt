package api

import (
	"encoding/json"
	"net/http"

	"github.com/braymix/shika/internal/models"
)

// modelView is a catalog entry enriched with this mesh's live state.
type modelView struct {
	models.Model
	Installed bool             `json:"installed"`
	Fits      bool             `json:"fits"`
	Verified  bool             `json:"verified"`
	Progress  *models.Progress `json:"progress,omitempty"`
}

// handleModels lists the curated catalog, annotated with whether each model is
// already downloaded, fits the combined mesh memory, and any download progress.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	meshRAM := s.meshRAMGB()
	views := make([]modelView, 0)
	for _, m := range models.Catalog() {
		v := modelView{
			Model:     m,
			Installed: s.mdl.Installed(m),
			Fits:      m.FitsMesh(meshRAM),
			Verified:  m.Verified(),
		}
		if p, ok := s.mdl.Progress(m.ID); ok {
			pp := p
			v.Progress = &pp
		}
		views = append(views, v)
	}
	current := s.cfg.Model
	if plan, ok := s.Plan(); ok && plan.Model != "" {
		current = plan.Model // the head's cluster-wide choice
	}
	writeJSON(w, map[string]any{
		"mesh_ram_gb": meshRAM,
		"current":     current,
		"models":      views,
	})
}

// handleModelSelect makes a catalog model this node's advertised choice. On the
// head that becomes the whole mesh's model (every node rebuilds its command for
// it and auto-downloads the file); on a worker it takes effect only if that
// worker later becomes head.
func (s *Server) handleModelSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = body.ID
	}
	m, ok := models.Find(id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown model id: "+id)
		return
	}
	s.reg.SetSelfModel(m.File)
	// Fetch it locally too if we don't have it yet.
	if !s.mdl.Installed(m) {
		s.mdl.Start(m)
	}
	writeJSON(w, map[string]any{"ok": true, "id": id, "file": m.File})
}

// EnsureModel downloads the mesh's currently chosen model on this node if it is
// a known catalog entry we don't already have. Safe to call repeatedly; an
// in-flight or completed download is a no-op. This is how a worker converges on
// the head's model without manual steps.
func (s *Server) EnsureModel() {
	plan, ok := s.Plan()
	if !ok || plan.Model == "" {
		return
	}
	for _, m := range models.Catalog() {
		if m.File != plan.Model {
			continue
		}
		if s.mdl.Installed(m) {
			return
		}
		if p, ok := s.mdl.Progress(m.ID); ok && (p.Active || p.Completed) {
			return
		}
		s.mdl.Start(m)
		return
	}
}

// handleModelDownload starts (or no-ops if already running) a background
// download of the catalog model named by the "id" field.
func (s *Server) handleModelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = body.ID
	}
	m, ok := models.Find(id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown model id: "+id)
		return
	}
	s.mdl.Start(m)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}
