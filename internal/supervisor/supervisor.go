// Package supervisor launches and watches this node's prima.cpp process
// according to the current cluster plan. It keeps the process matched to the
// desired command, restarts it with exponential backoff if it crashes, and —
// for the head — probes the LLM endpoint so the dashboard can report whether
// inference is actually answering, not merely that a process is alive.
package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/braymix/shika/internal/cluster"
	"github.com/braymix/shika/internal/health"
)

// Backoff bounds for restarting a crashed child.
const (
	backoffMin = 1 * time.Second
	backoffMax = 30 * time.Second

	healthEvery   = 3 * time.Second
	healthTimeout = 2 * time.Second

	// readinessEvery is how often the head re-checks whether its workers are up
	// before it launches llama-server.
	readinessEvery = 1 * time.Second
)

// State is a snapshot of what the supervisor is doing, for the API/dashboard.
type State struct {
	Running   bool     `json:"running"`
	DryRun    bool     `json:"dry_run"` // true when prima.cpp isn't built yet
	Waiting   bool     `json:"waiting"` // head is holding for workers to come up
	Role      string   `json:"role"`    // "head", "worker", or ""
	Command   []string `json:"command"`
	StartedAt string   `json:"started_at,omitempty"`
	Restarts  int      `json:"restarts"`
	LastExit  string   `json:"last_exit,omitempty"`
	LastError string   `json:"last_error,omitempty"`

	// Health reflects the most recent LLM endpoint probe. Only meaningful on the
	// head while it is running; the zero value (ok=false, no detail) means
	// "not probed".
	Health health.Result `json:"health"`
}

// Supervisor owns at most one prima.cpp child process and the goroutine that
// keeps it alive.
type Supervisor struct {
	primaDir string
	selfID   string

	mu        sync.Mutex
	state     State
	desired   []string           // command the active generation is driving
	active    bool               // a run loop is currently managing this node
	cancel    context.CancelFunc // cancels the active generation
	cmd       *exec.Cmd
	readiness func() bool // optional gate: head waits until this returns true
}

// New creates a supervisor for the given prima.cpp checkout and node id.
func New(primaDir, selfID string) *Supervisor {
	return &Supervisor{primaDir: primaDir, selfID: selfID}
}

// SetReadiness installs an optional gate consulted before the head launches
// llama-server, so workers come up before the head. Workers are never gated.
// Pass nil to launch the head immediately.
func (s *Supervisor) SetReadiness(fn func() bool) {
	s.mu.Lock()
	s.readiness = fn
	s.mu.Unlock()
}

// State returns a copy of the current state.
func (s *Supervisor) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Apply (re)launches prima.cpp for this node's role in the plan. It is safe to
// call repeatedly with the same plan: an unchanged command for this node is a
// no-op. A changed command tears down the current generation and starts a fresh
// supervised one.
func (s *Supervisor) Apply(ctx context.Context, plan cluster.Plan) {
	member, ok := findSelf(plan, s.selfID)
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active && equal(s.desired, member.Command) {
		return // already driving exactly this command
	}
	s.stopLocked()

	role := "worker"
	if member.IsHead {
		role = "head"
	}
	s.desired = member.Command
	s.state = State{Role: role, Command: member.Command}

	// If prima.cpp is not built yet, stay in dry-run: record the command we
	// *would* run so the operator can see it, but never exec anything and never
	// spawn a run loop.
	bin := strings.TrimPrefix(member.Command[0], "./")
	if !fileExists(filepath.Join(s.primaDir, bin)) {
		s.state.DryRun = true
		return
	}

	genCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.active = true
	go s.run(genCtx, member, plan.LLMURL)
}

// run supervises one generation of the child: optional readiness gating for the
// head, then a start/wait/backoff loop until the generation is cancelled.
func (s *Supervisor) run(ctx context.Context, member cluster.Member, llmURL string) {
	if member.IsHead {
		if !s.waitForReadiness(ctx) {
			return // cancelled while waiting
		}
	}
	if member.IsHead && llmURL != "" {
		go s.probeHealth(ctx, llmURL)
	}

	backoff := backoffMin
	for {
		if ctx.Err() != nil {
			return
		}
		exited, started := s.startAndWait(ctx, member.Command)
		if ctx.Err() != nil {
			return // a stop/replan, not a crash
		}
		// Unexpected exit: record it and back off before retrying. A process
		// that stayed up a while resets the backoff.
		s.mu.Lock()
		s.state.Running = false
		s.state.Restarts++
		s.state.LastExit = exited
		s.mu.Unlock()
		if time.Since(started) > backoffMax {
			backoff = backoffMin
		}
		if !sleepCtx(ctx, backoff) {
			return
		}
		backoff *= 2
		if backoff > backoffMax {
			backoff = backoffMax
		}
	}
}

// waitForReadiness blocks the head until its readiness gate reports true (all
// workers up). Returns false if the generation is cancelled first.
func (s *Supervisor) waitForReadiness(ctx context.Context) bool {
	s.mu.Lock()
	fn := s.readiness
	s.mu.Unlock()
	if fn == nil || fn() {
		return true
	}
	s.mu.Lock()
	s.state.Waiting = true
	s.mu.Unlock()

	t := time.NewTicker(readinessEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
			if fn() {
				s.mu.Lock()
				s.state.Waiting = false
				s.mu.Unlock()
				return true
			}
		}
	}
}

// startAndWait launches the child and blocks until it exits or ctx is done.
// It returns a short exit description and the time the child started.
func (s *Supervisor) startAndWait(ctx context.Context, command []string) (exit string, started time.Time) {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = s.primaDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setProcAttr(cmd)
	// CommandContext defaults to SIGKILL on the process only; replace it with a
	// group kill so ctx cancellation also reaps forked helpers.
	cmd.Cancel = func() error { killProc(cmd); return nil }

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.state.LastError = err.Error()
		s.mu.Unlock()
		return "start failed: " + err.Error(), time.Now()
	}

	started = time.Now()
	s.mu.Lock()
	s.cmd = cmd
	s.state.Running = true
	s.state.Waiting = false
	s.state.LastError = ""
	s.state.StartedAt = started.Format(time.RFC3339)
	s.mu.Unlock()

	err := cmd.Wait()

	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
	}
	s.mu.Unlock()

	if ctx.Err() != nil {
		return "stopped", started
	}
	if err != nil {
		return "exited: " + err.Error(), started
	}
	return "exited cleanly", started
}

// probeHealth periodically checks the head LLM endpoint and records the result.
func (s *Supervisor) probeHealth(ctx context.Context, llmURL string) {
	t := time.NewTicker(healthEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			res := health.ProbeLLM(ctx, llmURL, healthTimeout)
			s.mu.Lock()
			s.state.Health = res
			s.mu.Unlock()
		}
	}
}

// Stop terminates any running child and ends the active generation.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	s.desired = nil
	s.state = State{}
}

// stopLocked cancels the current generation and kills its child. Caller holds mu.
func (s *Supervisor) stopLocked() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.cmd != nil {
		killProc(s.cmd)
		s.cmd = nil
	}
	s.active = false
	s.state.Running = false
	s.state.Waiting = false
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

// sleepCtx sleeps for d unless ctx is cancelled first. Returns false if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
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
