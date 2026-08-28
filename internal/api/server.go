// Package api serves the local control API and the device-management dashboard.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/braymix/shika/internal/cluster"
	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/models"
	"github.com/braymix/shika/internal/supervisor"
	"github.com/braymix/shika/web"
)

// peerStateClient fetches a peer's /api/state. Short timeout so a slow or dead
// peer never stalls the head's readiness check.
var peerStateClient = &http.Client{Timeout: 2 * time.Second}

// Server wires the registry, planner and supervisor to HTTP handlers.
type Server struct {
	cfg config.Config
	reg *discovery.Registry
	sup *supervisor.Supervisor
	mdl *models.Manager

	mu        sync.RWMutex
	autostart bool
}

// New builds a Server. autostart reflects cfg.AutoStart initially.
func New(cfg config.Config, reg *discovery.Registry, sup *supervisor.Supervisor) *Server {
	return &Server{
		cfg:       cfg,
		reg:       reg,
		sup:       sup,
		mdl:       models.NewManager(cfg.PrimaDir),
		autostart: cfg.AutoStart,
	}
}

// meshRAMGB is the combined memory of all alive nodes, used to tell the operator
// whether a model fits the current mesh.
func (s *Server) meshRAMGB() float64 {
	var bytes uint64
	for _, p := range s.reg.Alive() {
		bytes += p.RAMBytes
	}
	return float64(bytes) / (1 << 30)
}

// AutoStart reports whether the operator has enabled automatic launch.
func (s *Server) AutoStart() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autostart
}

// Plan computes the current deterministic plan from live membership.
func (s *Server) Plan() (cluster.Plan, bool) {
	return cluster.Build(s.reg.Alive(), s.cfg)
}

// WorkersReady reports whether it is safe for the head to launch: every worker
// in the current plan has its prima.cpp process running. It is meant to be
// handed to supervisor.SetReadiness so the head comes up only after its workers
// (prima.cpp needs the ring of workers listening before rank 0 starts).
//
// If this node is not the head, there is nothing to gate and it returns true.
// A worker we cannot reach counts as not-ready, so the head keeps waiting
// rather than launching into a half-formed ring.
func (s *Server) WorkersReady() bool {
	plan, ok := s.Plan()
	if !ok {
		return false
	}
	selfID := s.reg.Self().ID

	amHead := false
	for _, m := range plan.Members {
		if m.Info.ID == selfID {
			amHead = m.IsHead
		}
	}
	if !amHead {
		return true
	}

	for _, m := range plan.Members {
		if m.IsHead {
			continue
		}
		if m.Info.ID == selfID {
			continue // shouldn't happen for the head, but be safe
		}
		if !peerRunning(m.Info.Control) {
			return false
		}
	}
	return true
}

// peerRunning fetches a peer's /api/state and reports whether its supervisor is
// currently running a process.
func peerRunning(control string) bool {
	if control == "" {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, "http://"+control+"/api/state", nil)
	if err != nil {
		return false
	}
	resp, err := peerStateClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var payload struct {
		State supervisor.State `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false
	}
	return payload.State.Running
}

// Handler returns the root http.Handler (API + dashboard).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/self", s.handleSelf)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/plan", s.handlePlan)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/cluster/start", s.handleStart)
	mux.HandleFunc("/api/cluster/stop", s.handleStop)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/webui", s.handleWebUI)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/models/download", s.handleModelDownload)
	mux.Handle("/", http.FileServer(http.FS(web.FS())))
	return logging(mux)
}

func (s *Server) handleSelf(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.reg.Self())
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.reg.Alive())
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	plan, ok := s.Plan()
	writeJSON(w, map[string]any{"ok": ok, "plan": plan})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"state":     s.sup.State(),
		"autostart": s.AutoStart(),
		"node":      s.reg.Self(),
	})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	s.autostart = true
	s.mu.Unlock()
	if plan, ok := s.Plan(); ok {
		s.sup.Apply(context.Background(), plan)
	}
	writeJSON(w, map[string]any{"ok": true, "autostart": true})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	s.autostart = false
	s.mu.Unlock()
	s.sup.Stop()
	writeJSON(w, map[string]any{"ok": true, "autostart": false})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start // hook point for real request logging later
	})
}
