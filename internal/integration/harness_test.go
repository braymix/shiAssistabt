// Package integration exercises the whole control plane wired together the way
// it runs in production — several full nodes (registry + control API +
// supervisor) talking over loopback via seed discovery — without needing real
// devices or a real prima.cpp build. Fake executables stand in for prima.cpp.
package integration

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/braymix/shika/internal/api"
	"github.com/braymix/shika/internal/cluster"
	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
	"github.com/braymix/shika/internal/supervisor"
)

// testNode is one fully wired shikA daemon serving on a loopback port.
type testNode struct {
	self node.Info
	reg  *discovery.Registry
	sup  *supervisor.Supervisor
	srv  *api.Server
}

// startNode brings up a node listening on 127.0.0.1:0 with a prima.cpp dir that
// contains fake llama-server/llama-cli scripts, and begins serving its API.
func startNode(t *testing.T, ctx context.Context, name string, ramGB float64, cores int) *testNode {
	t.Helper()

	dir := t.TempDir()
	writeFakeBinaries(t, dir)

	// Grab a concrete port first so self.Control can advertise it before we serve.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host := ln.Addr().String()

	self := node.Info{
		ID: name, Name: name, OS: "linux", Arch: "amd64",
		Cores: cores, RAMBytes: uint64(ramGB * (1 << 30)),
		Control: host, LLMPort: 8080,
	}
	cfg := config.Default()
	cfg.PrimaDir = dir

	reg := discovery.NewRegistry(self, 10*time.Second)
	sup := supervisor.New(dir, self.ID)
	srv := api.New(cfg, reg, sup)
	sup.SetReadiness(srv.WorkersReady)

	hs := &http.Server{Handler: srv.Handler()}
	go func() { _ = hs.Serve(ln) }()
	go func() {
		<-ctx.Done()
		sup.Stop()
		_ = hs.Close()
	}()

	return &testNode{self: self, reg: reg, sup: sup, srv: srv}
}

// writeFakeBinaries drops long-sleeping stand-ins for prima.cpp so the
// supervisor takes its real exec path instead of dry-run.
func writeFakeBinaries(t *testing.T, dir string) {
	t.Helper()
	for _, bin := range []string{"llama-server", "llama-cli", "rpc-server"} {
		p := filepath.Join(dir, bin)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", bin, err)
		}
	}
}

func hosts(nodes ...*testNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.self.Control
	}
	return out
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestSeedDiscoveryConvergesToIdenticalPlan is the core distributed guarantee:
// independent nodes that only know each other through seeds must each compute
// the *same* plan, with no central coordinator.
func TestSeedDiscoveryConvergesToIdenticalPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("integration harness skipped in -short mode")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := startNode(t, ctx, "alpha", 16, 8) // most RAM -> head
	b := startNode(t, ctx, "bravo", 8, 8)
	c := startNode(t, ctx, "delta", 4, 4)

	// Each node polls the others over HTTP, exactly like Tailscale/remote peers.
	go discovery.RunSeeds(ctx, a.reg, hosts(b, c), 150*time.Millisecond)
	go discovery.RunSeeds(ctx, b.reg, hosts(a, c), 150*time.Millisecond)
	go discovery.RunSeeds(ctx, c.reg, hosts(a, b), 150*time.Millisecond)

	nodes := []*testNode{a, b, c}
	waitFor(t, 5*time.Second, "all nodes to see 3 members", func() bool {
		for _, n := range nodes {
			p, ok := n.srv.Plan()
			if !ok || p.World != 3 {
				return false
			}
		}
		return true
	})

	// Every node must agree on head and per-rank commands.
	var ref cluster.Plan
	for i, n := range nodes {
		p, _ := n.srv.Plan()
		if i == 0 {
			ref = p
			continue
		}
		if p.HeadID != ref.HeadID {
			t.Fatalf("node %s elected head %q, want %q", n.self.Name, p.HeadID, ref.HeadID)
		}
		if !plansMatch(ref, p) {
			t.Fatalf("node %s computed a different plan than %s", n.self.Name, nodes[0].self.Name)
		}
	}
	if ref.HeadID != "alpha" {
		t.Fatalf("head = %q, want alpha (most RAM)", ref.HeadID)
	}
}

// TestAutostartBringsUpHeadAfterWorkers verifies the end-to-end ordering: with
// autostart on, workers launch and the head only comes up once its workers'
// processes are running (readiness gated over the control API).
func TestAutostartBringsUpHeadAfterWorkers(t *testing.T) {
	if testing.Short() {
		t.Skip("integration harness skipped in -short mode")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	head := startNode(t, ctx, "head", 32, 16)
	worker := startNode(t, ctx, "worker", 8, 4)

	go discovery.RunSeeds(ctx, head.reg, hosts(worker), 150*time.Millisecond)
	go discovery.RunSeeds(ctx, worker.reg, hosts(head), 150*time.Millisecond)

	nodes := []*testNode{head, worker}
	waitFor(t, 5*time.Second, "both nodes to see 2 members", func() bool {
		for _, n := range nodes {
			p, ok := n.srv.Plan()
			if !ok || p.World != 2 {
				return false
			}
		}
		return true
	})

	// Drive each node's reconcile loop the way cmd/shikad does.
	for _, n := range nodes {
		go reconcile(ctx, n)
	}

	waitFor(t, 8*time.Second, "worker prima.cpp to be running", func() bool {
		return worker.sup.State().Running
	})
	waitFor(t, 8*time.Second, "head prima.cpp to be running after readiness", func() bool {
		return head.sup.State().Running
	})

	// The head's endpoint should be the strongest node.
	p, _ := head.srv.Plan()
	if p.HeadID != "head" {
		t.Fatalf("head = %q, want head (most RAM)", p.HeadID)
	}
}

func reconcile(ctx context.Context, n *testNode) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if plan, ok := n.srv.Plan(); ok {
				n.sup.Apply(ctx, plan)
			}
		}
	}
}

func plansMatch(a, b cluster.Plan) bool {
	if a.World != b.World || a.HeadID != b.HeadID || a.HeadIP != b.HeadIP || a.LLMURL != b.LLMURL {
		return false
	}
	if len(a.Members) != len(b.Members) {
		return false
	}
	for i := range a.Members {
		ma, mb := a.Members[i], b.Members[i]
		if ma.Info.ID != mb.Info.ID || ma.Rank != mb.Rank || ma.IsHead != mb.IsHead {
			return false
		}
		if len(ma.Command) != len(mb.Command) {
			return false
		}
		for j := range ma.Command {
			if ma.Command[j] != mb.Command[j] {
				return false
			}
		}
	}
	return true
}
