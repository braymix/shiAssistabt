package api

import (
	"context"
	"runtime"

	"github.com/braymix/shika/internal/engine"
)

// engineStatus is the provisioning state of the inference engine, shown on the
// dashboard so a plain download can fetch prima.cpp on first Start.
type engineStatus struct {
	Status string `json:"status"` // missing | downloading | ready | error
	Done   int64  `json:"done"`
	Total  int64  `json:"total"`
	Error  string `json:"error,omitempty"`
}

// engineDir is the working directory the engine binaries live in.
func (s *Server) engineDir() string { return s.cfg.PrimaDir }

// EngineStatus reports the current engine provisioning state.
func (s *Server) EngineStatus() engineStatus {
	s.engMu.Lock()
	defer s.engMu.Unlock()
	st := s.eng
	if st.Status == "" || st.Status == "missing" {
		if runtime.GOOS == "android" {
			// The engine ships inside the APK; presence is whatever the app bundled.
			st.Status = "ready"
		} else if engine.Installed(s.engineDir()) {
			st.Status = "ready"
		} else {
			st.Status = "missing"
		}
	}
	return st
}

// EnsureEngine downloads the engine bundle for this platform if it isn't already
// present, in the background. Safe to call repeatedly. A no-op on Android (the
// engine is bundled in the APK) and when already installed or downloading.
func (s *Server) EnsureEngine() {
	if runtime.GOOS == "android" {
		return
	}
	if engine.Installed(s.engineDir()) {
		s.setEngine(engineStatus{Status: "ready"})
		return
	}
	s.engMu.Lock()
	if s.eng.Status == "downloading" {
		s.engMu.Unlock()
		return
	}
	s.eng = engineStatus{Status: "downloading"}
	s.engMu.Unlock()

	go func() {
		ctx := context.Background()
		url, err := engine.LatestAssetURL(ctx, nil)
		if err != nil {
			s.setEngine(engineStatus{Status: "error", Error: err.Error()})
			return
		}
		err = engine.Ensure(ctx, s.engineDir(), url, func(done, total int64) {
			s.engMu.Lock()
			s.eng.Done, s.eng.Total = done, total
			s.engMu.Unlock()
		})
		if err != nil {
			s.setEngine(engineStatus{Status: "error", Error: err.Error()})
			return
		}
		s.setEngine(engineStatus{Status: "ready"})
	}()
}

func (s *Server) setEngine(st engineStatus) {
	s.engMu.Lock()
	s.eng = st
	s.engMu.Unlock()
}
