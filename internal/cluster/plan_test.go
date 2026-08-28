package cluster

import (
	"testing"
	"time"

	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
)

func peer(id, name, control string, ramGB float64, cores int) discovery.Peer {
	return discovery.Peer{
		Info: node.Info{
			ID: id, Name: name, Control: control, LLMPort: 8080,
			RAMBytes: uint64(ramGB * 1024 * 1024 * 1024), Cores: cores,
		},
		LastSeen: time.Now(),
	}
}

func TestBuildElectsStrongestHead(t *testing.T) {
	cfg := config.Default()
	peers := []discovery.Peer{
		peer("a", "phone", "192.168.1.3:8977", 6, 8),
		peer("b", "mac", "192.168.1.2:8977", 16, 8), // most RAM -> head
		peer("c", "pi", "192.168.1.4:8977", 4, 4),
	}
	plan, ok := Build(peers, cfg)
	if !ok {
		t.Fatal("expected ok plan")
	}
	if plan.World != 3 {
		t.Fatalf("world = %d, want 3", plan.World)
	}
	if plan.HeadID != "b" {
		t.Fatalf("head = %q, want b (most RAM)", plan.HeadID)
	}
	if plan.Members[0].Rank != 0 || !plan.Members[0].IsHead {
		t.Fatal("rank 0 must be the head")
	}
	if plan.LLMURL != "http://192.168.1.2:8080/v1" {
		t.Fatalf("llm url = %q", plan.LLMURL)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	cfg := config.Default()
	set1 := []discovery.Peer{
		peer("a", "n1", "10.0.0.1:8977", 8, 4),
		peer("b", "n2", "10.0.0.2:8977", 8, 4),
	}
	set2 := []discovery.Peer{ // same nodes, reversed order
		peer("b", "n2", "10.0.0.2:8977", 8, 4),
		peer("a", "n1", "10.0.0.1:8977", 8, 4),
	}
	p1, _ := Build(set1, cfg)
	p2, _ := Build(set2, cfg)
	if p1.HeadID != p2.HeadID {
		t.Fatalf("non-deterministic head: %q vs %q", p1.HeadID, p2.HeadID)
	}
}

func TestBuildRingClosesAndCommandsAreRoleCorrect(t *testing.T) {
	cfg := config.Default()
	peers := []discovery.Peer{
		peer("a", "big", "192.168.1.2:8977", 32, 16),
		peer("b", "small", "192.168.1.3:8977", 8, 4),
	}
	plan, _ := Build(peers, cfg)

	// ring: last member's next must be the head's IP
	last := plan.Members[len(plan.Members)-1]
	if last.NextIP != plan.HeadIP {
		t.Fatalf("ring not closed: last.next=%q head=%q", last.NextIP, plan.HeadIP)
	}
	// head runs llama-server, worker runs llama-cli
	if plan.Members[0].Command[0] != "./llama-server" {
		t.Fatalf("head cmd = %v", plan.Members[0].Command)
	}
	if plan.Members[1].Command[0] != "./llama-cli" {
		t.Fatalf("worker cmd = %v", plan.Members[1].Command)
	}
}

func TestBuildEmpty(t *testing.T) {
	if _, ok := Build(nil, config.Default()); ok {
		t.Fatal("empty peer set should not produce a plan")
	}
}
