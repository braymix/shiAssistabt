// Package cluster turns the set of alive peers into a concrete, deterministic
// prima.cpp launch plan (who is head, the ring order, and each node's command).
package cluster

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
)

// Member is one node's assigned role within a plan.
type Member struct {
	Info    node.Info `json:"info"`
	Rank    int       `json:"rank"`    // 0 == head
	IP      string    `json:"ip"`      // control host, used as prima.cpp address
	NextIP  string    `json:"next_ip"` // ring: address of the next rank
	IsHead  bool      `json:"is_head"`
	Command []string  `json:"command"` // prima.cpp argv this node should run
}

// Plan is the full cluster assignment, identical on every node because it is a
// pure function of the (sorted) member set.
type Plan struct {
	World   int      `json:"world"`
	HeadID  string   `json:"head_id"`
	HeadIP  string   `json:"head_ip"`
	Model   string   `json:"model"`   // GGUF filename every node runs (chosen by the head)
	LLMURL  string   `json:"llm_url"` // OpenAI-compatible base URL to feed Open WebUI
	Members []Member `json:"members"`
}

// Build computes a deterministic plan from the alive peers. Every node runs
// this over the same membership and arrives at the same result, so no central
// coordinator is required. Returns ok=false if there are no usable peers.
func Build(peers []discovery.Peer, cfg config.Config) (Plan, bool) {
	if len(peers) == 0 {
		return Plan{}, false
	}

	// Sort strongest-first; tie-break on ID for stable, identical ordering
	// across all nodes.
	sort.Slice(peers, func(a, b int) bool {
		ca, cb := peers[a].Capability(), peers[b].Capability()
		if ca != cb {
			return ca > cb
		}
		return peers[a].ID < peers[b].ID
	})

	world := len(peers)
	head := peers[0]
	headIP := hostOf(head.Control)

	// The model is a cluster-wide choice made by the head and advertised in its
	// beacon, so every node builds the same commands. Fall back to this node's
	// configured default until the head advertises one.
	model := head.Model
	if model == "" {
		model = cfg.Model
	}

	members := make([]Member, world)
	for i := range peers {
		ip := hostOf(peers[i].Control)
		nextIP := hostOf(peers[(i+1)%world].Control)
		members[i] = Member{
			Info:   peers[i].Info,
			Rank:   i,
			IP:     ip,
			NextIP: nextIP,
			IsHead: i == 0,
		}
	}
	for i := range members {
		members[i].Command = buildCommand(members[i], headIP, world, model, cfg)
	}

	return Plan{
		World:   world,
		HeadID:  head.ID,
		HeadIP:  headIP,
		Model:   model,
		LLMURL:  fmt.Sprintf("http://%s:%d/v1", headIP, head.LLMPort),
		Members: members,
	}, true
}

// buildCommand assembles the prima.cpp argv for a member. The head runs
// llama-server (persistent OpenAI API + web UI); workers run llama-cli.
func buildCommand(m Member, headIP string, world int, modelFile string, cfg config.Config) []string {
	model := modelPath(cfg.ModelDir, modelFile)
	common := []string{
		"--world", itoa(world),
		"--rank", itoa(m.Rank),
		"--master", headIP,
		"--next", m.NextIP,
		"--data-port", itoa(cfg.DataPort),
		"--signal-port", itoa(cfg.SignalPort),
		"--prefetch",
	}
	if m.IsHead {
		argv := []string{
			cfg.ServerBin, "-m", model,
			"--host", "0.0.0.0", "--port", itoa(m.Info.LLMPort),
			"-c", "2048",
		}
		return append(argv, common...)
	}
	argv := []string{cfg.CliBin, "-m", model}
	return append(argv, common...)
}

// modelPath resolves the -m argument. An absolute ModelDir is used verbatim
// (Android's writable storage); a relative one keeps prima.cpp's cwd-relative
// "download/<file>" form.
func modelPath(modelDir, file string) string {
	if modelDir == "" {
		modelDir = "download"
	}
	if filepath.IsAbs(modelDir) {
		return filepath.Join(modelDir, file)
	}
	return modelDir + "/" + file
}

// hostOf extracts the host part of a host:port string. If there is no port it
// returns the input unchanged.
func hostOf(hostport string) string {
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			return hostport[:i]
		}
	}
	return hostport
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
