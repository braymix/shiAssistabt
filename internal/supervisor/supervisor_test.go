package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/braymix/shika/internal/cluster"
	"github.com/braymix/shika/internal/node"
)

// writeScript drops an executable shell script named bin into dir.
func writeScript(t *testing.T, dir, bin, body string) {
	t.Helper()
	path := filepath.Join(dir, bin)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", bin, err)
	}
}

// planFor wraps a single self-member command into a plan the supervisor accepts.
func planFor(id string, isHead bool, command []string, llmURL string) cluster.Plan {
	return cluster.Plan{
		LLMURL: llmURL,
		Members: []cluster.Member{{
			Info:    node.Info{ID: id},
			IsHead:  isHead,
			Command: command,
		}},
	}
}

// waitFor polls cond up to timeout; fails the test if it never holds.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestApplyDryRunWhenBinaryMissing(t *testing.T) {
	sup := New(t.TempDir(), "self")
	sup.Apply(context.Background(), planFor("self", false, []string{"./llama-cli", "-m", "x"}, ""))

	st := sup.State()
	if !st.DryRun {
		t.Fatalf("expected dry-run, got %+v", st)
	}
	if st.Running {
		t.Fatal("dry-run must not run a process")
	}
	if st.Role != "worker" {
		t.Fatalf("role = %q, want worker", st.Role)
	}
}

func TestSupervisorRestartsOnCrash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script helper is POSIX-only")
	}
	dir := t.TempDir()
	writeScript(t, dir, "crasher", "exit 1")

	sup := New(dir, "self")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sup.Apply(ctx, planFor("self", false, []string{"./crasher"}, ""))

	// The child exits immediately; the supervisor should record at least one
	// restart well before the first backoff (1s) elapses a second time.
	waitFor(t, 3*time.Second, "a restart to be recorded", func() bool {
		return sup.State().Restarts >= 1
	})
	if got := sup.State().LastExit; got == "" {
		t.Fatal("expected a LastExit description after a crash")
	}
	sup.Stop()
	if sup.State().Running {
		t.Fatal("Stop should leave the supervisor not running")
	}
}

func TestHeadWaitsForReadiness(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script helper is POSIX-only")
	}
	dir := t.TempDir()
	writeScript(t, dir, "llama-server", "sleep 30")

	sup := New(dir, "self")
	ready := make(chan struct{})
	sup.SetReadiness(func() bool {
		select {
		case <-ready:
			return true
		default:
			return false
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sup.Apply(ctx, planFor("self", true, []string{"./llama-server"}, ""))

	// While workers are not ready, the head holds and does not launch.
	waitFor(t, time.Second, "head to enter waiting", func() bool {
		return sup.State().Waiting
	})
	if sup.State().Running {
		t.Fatal("head must not run before readiness")
	}

	// Once workers are up, it launches.
	close(ready)
	waitFor(t, 3*time.Second, "head to start after readiness", func() bool {
		st := sup.State()
		return st.Running && !st.Waiting
	})
	sup.Stop()
}

func TestApplySameCommandIsNoOp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script helper is POSIX-only")
	}
	dir := t.TempDir()
	writeScript(t, dir, "llama-cli", "sleep 30")

	sup := New(dir, "self")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := []string{"./llama-cli", "-m", "model"}
	sup.Apply(ctx, planFor("self", false, cmd, ""))
	waitFor(t, 2*time.Second, "first launch", func() bool { return sup.State().Running })
	started := sup.State().StartedAt

	// Re-applying the identical command must not restart the process.
	sup.Apply(ctx, planFor("self", false, cmd, ""))
	time.Sleep(200 * time.Millisecond)
	st := sup.State()
	if st.StartedAt != started {
		t.Fatalf("identical Apply restarted the child: %q -> %q", started, st.StartedAt)
	}
	if st.Restarts != 0 {
		t.Fatalf("no-op Apply should not count a restart, got %d", st.Restarts)
	}
	sup.Stop()
}
