// Package cluster turns the set of alive peers into a concrete, deterministic
// llama.cpp launch plan: who is head (runs llama-server), and — when there is
// more than one device — which workers run rpc-server so the head can spread the
// model across all of them (llama.cpp's RPC "shared power").
package cluster

import (
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"

	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
)

// Member is one node's assigned role within a plan.
type Member struct {
	Info    node.Info `json:"info"`
	Rank    int       `json:"rank"` // 0 == head
	IP      string    `json:"ip"`   // this node's address
	IsHead  bool      `json:"is_head"`
	Command []string  `json:"command"` // engine argv this node should run
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
		members[i] = Member{
			Info:   peers[i].Info,
			Rank:   i,
			IP:     hostOf(peers[i].Control),
			IsHead: i == 0,
		}
	}

	// Workers expose an rpc-server the head offloads model layers to. Collect
	// their endpoints for the head's --rpc list.
	rpcPort := cfg.RpcPort
	workerRPCs := make([]string, 0, world-1)
	for i := 1; i < world; i++ {
		workerRPCs = append(workerRPCs, net.JoinHostPort(members[i].IP, itoa(rpcPort)))
	}

	for i := range members {
		members[i].Command = buildCommand(members[i], model, cfg, workerRPCs)
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

// buildCommand assembles the llama.cpp argv for a member. The head runs
// llama-server (the OpenAI-compatible endpoint) and loads the model; with more
// than one device it offloads layers to the workers over RPC. Each worker runs
// rpc-server and needs no model file of its own.
func buildCommand(m Member, modelFile string, cfg config.Config, workerRPCs []string) []string {
	if m.IsHead {
		argv := []string{
			cfg.ServerBin, "-m", modelPath(cfg.ModelDir, modelFile),
			"--host", "0.0.0.0", "--port", itoa(m.Info.LLMPort),
			"-c", "4096",
		}
		if len(workerRPCs) > 0 {
			argv = append(argv, "--rpc", strings.Join(workerRPCs, ","))
		}
		return argv
	}
	// Worker: a compute/memory node for the head's RPC.
	return []string{cfg.RpcBin, "-H", "0.0.0.0", "-p", itoa(cfg.RpcPort)}
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
