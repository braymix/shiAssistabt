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

func TestBuildUsesHeadAdvertisedModel(t *testing.T) {
	cfg := config.Default() // default model differs from what the head advertises
	head := peer("h", "head", "192.168.1.2:8977", 32, 16)
	head.Model = "llama3.1-8b.gguf" // head's cluster-wide choice
	worker := peer("w", "worker", "192.168.1.3:8977", 8, 4)
	// worker still advertises the default; the head's choice must win for all.

	plan, ok := Build([]discovery.Peer{worker, head}, cfg)
	if !ok {
		t.Fatal("expected ok plan")
	}
	if plan.Model != "llama3.1-8b.gguf" {
		t.Fatalf("plan.Model = %q, want the head's model", plan.Model)
	}
	for _, m := range plan.Members {
		if !containsArg(m.Command, "download/llama3.1-8b.gguf") {
			t.Fatalf("member %s command uses wrong model: %v", m.Info.Name, m.Command)
		}
	}
}

func TestBuildFallsBackToConfigModel(t *testing.T) {
	cfg := config.Default()
	// No peer advertises a model, so every command uses the configured default.
	plan, _ := Build([]discovery.Peer{peer("a", "a", "10.0.0.1:8977", 8, 4)}, cfg)
	if plan.Model != cfg.Model {
		t.Fatalf("plan.Model = %q, want config default %q", plan.Model, cfg.Model)
	}
}

func TestBuildHonorsAndroidEnginePaths(t *testing.T) {
	// Android points the engine at bundled native libs and a writable model dir.
	cfg := config.Default()
	cfg.ServerBin = "./libllama-server.so"
	cfg.CliBin = "./libllama-cli.so"
	cfg.ModelDir = "/data/data/com.shika.app/files/models"

	peers := []discovery.Peer{
		peer("h", "phoneA", "192.168.43.1:8977", 8, 8),
		peer("w", "phoneB", "192.168.43.20:8977", 6, 8),
	}
	plan, _ := Build(peers, cfg)

	head := plan.Members[0]
	if head.Command[0] != "./libllama-server.so" {
		t.Fatalf("head bin = %q, want the bundled server lib", head.Command[0])
	}
	worker := plan.Members[1]
	if worker.Command[0] != "./libllama-cli.so" {
		t.Fatalf("worker bin = %q, want the bundled cli lib", worker.Command[0])
	}
	want := "/data/data/com.shika.app/files/models/" + plan.Model
	if !containsArg(head.Command, want) {
		t.Fatalf("head -m path not absolute writable dir: %v", head.Command)
	}
}

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}
