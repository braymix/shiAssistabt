// Package supervisor launches and watches this node's prima.cpp process
// according to the current cluster plan.
package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenCPIL/prima-mesh/internal/cluster"
)

// State is a snapshot of what the supervisor is doing, for the API/dashboard.
type State struct {
	Running   bool     `json:"running"`
	DryRun    bool     `json:"dry_run"` // true when prima.cpp isn't built yet
	Role      string   `json:"role"`    // "head", "worker", or ""
	Command   []string `json:"command"`
	StartedAt string   `json:"started_at,omitempty"`
	LastError string   `json:"last_error,omitempty"`
}

// Supervisor owns at most one prima.cpp child process.
type Supervisor struct {
	primaDir string
	selfID   string

	mu    sync.Mutex
	cmd   *exec.Cmd
	state State
}

// New creates a supervisor for the given prima.cpp checkout and node id.
func New(primaDir, selfID string) *Supervisor {
	return &Supervisor{primaDir: primaDir, selfID: selfID}
}

// State returns a copy of the current state.
func (s *Supervisor) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Apply (re)launches prima.cpp for this node's role in the plan. If the plan's
// command for this node is unchanged and already running, it is a no-op.
func (s *Supervisor) Apply(ctx context.Context, plan cluster.Plan) {
	member, ok := findSelf(plan, s.selfID)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	same := s.state.Running && equal(s.state.Command, member.Command)
	if same {
		return
	}
	s.stopLocked()

	role := "worker"
	if member.IsHead {
		role = "head"
	}
	s.state = State{Role: role, Command: member.Command}

	// If prima.cpp is not built yet, stay in dry-run: record the command we
	// *would* run so the operator can see it, but do not exec anything.
	bin := strings.TrimPrefix(member.Command[0], "./")
	if !fileExists(filepath.Join(s.primaDir, bin)) {
		s.state.DryRun = true
		return
	}

	cmd := exec.CommandContext(ctx, member.Command[0], member.Command[1:]...)
	cmd.Dir = s.primaDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		s.state.LastError = err.Error()
		return
	}
	s.cmd = cmd
	s.state.Running = true
	s.state.StartedAt = time.Now().Format(time.RFC3339)
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.state.Running = false
			s.cmd = nil
		}
		s.mu.Unlock()
	}()
}

// Stop terminates any running child.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *Supervisor) stopLocked() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		s.cmd = nil
	}
	s.state.Running = false
}

func findSelf(plan cluster.Plan, id string) (cluster.Member, bool) {
	for _, m := range plan.Members {
		if m.Info.ID == id {
			return m, true
		}
	}
	return cluster.Member{}, false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
