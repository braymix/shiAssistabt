// Package cluster turns the set of alive peers into a concrete, deterministic
// prima.cpp launch plan (who is head, the ring order, and each node's command).
package cluster

import (
	"fmt"
	"sort"

	"github.com/OpenCPIL/prima-mesh/internal/config"
	"github.com/OpenCPIL/prima-mesh/internal/discovery"
	"github.com/OpenCPIL/prima-mesh/internal/node"
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
		members[i].Command = buildCommand(members[i], headIP, world, cfg)
	}

	return Plan{
		World:   world,
		HeadID:  head.ID,
		HeadIP:  headIP,
		LLMURL:  fmt.Sprintf("http://%s:%d/v1", headIP, head.LLMPort),
		Members: members,
	}, true
}

// buildCommand assembles the prima.cpp argv for a member. The head runs
// llama-server (persistent OpenAI API + web UI); workers run llama-cli.
func buildCommand(m Member, headIP string, world int, cfg config.Config) []string {
	model := "download/" + cfg.Model
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
			"./llama-server", "-m", model,
			"--host", "0.0.0.0", "--port", itoa(m.Info.LLMPort),
			"-c", "2048",
		}
		return append(argv, common...)
	}
	argv := []string{"./llama-cli", "-m", model}
	return append(argv, common...)
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
