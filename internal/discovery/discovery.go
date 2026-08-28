// Package discovery finds other shikA nodes on the network.
//
// Two mechanisms:
//   - LAN: UDP multicast beacons (zero-config, works on the same Wi-Fi/subnet).
//   - Seeds: explicit control-API addresses polled over HTTP, for peers that
//     multicast cannot reach (e.g. across subnets or over Tailscale).
package discovery

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/braymix/shika/internal/node"
)

// Peer is a known node plus liveness bookkeeping.
type Peer struct {
	node.Info
	LastSeen time.Time `json:"last_seen"`
	Source   string    `json:"source"` // "lan" or "seed"
}

// Registry is a thread-safe set of known peers, including self.
type Registry struct {
	mu      sync.RWMutex
	self    node.Info
	peers   map[string]Peer
	timeout time.Duration
}

// NewRegistry creates a registry seeded with this node's own Info.
func NewRegistry(self node.Info, timeout time.Duration) *Registry {
	return &Registry{
		self:    self,
		peers:   make(map[string]Peer),
		timeout: timeout,
	}
}

// Self returns this node's Info.
func (r *Registry) Self() node.Info { return r.self }

func (r *Registry) upsert(info node.Info, source string) {
	if info.ID == "" || info.ID == r.self.ID {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[info.ID] = Peer{Info: info, LastSeen: time.Now(), Source: source}
}

// Alive returns self plus all peers seen within the timeout window, self first.
func (r *Registry) Alive() []Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Peer{{Info: r.self, LastSeen: time.Now(), Source: "self"}}
	cutoff := time.Now().Add(-r.timeout)
	for _, p := range r.peers {
		if p.LastSeen.After(cutoff) {
			out = append(out, p)
		}
	}
	return out
}

// multicast starts the beacon sender and listener. It blocks until ctx is done.
func RunMulticast(ctx context.Context, reg *Registry, addr string, every time.Duration) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Printf("discovery: bad multicast addr %q: %v", addr, err)
		return
	}
	go beacon(ctx, reg, udpAddr, every)
	listen(ctx, reg, udpAddr)
}

func beacon(ctx context.Context, reg *Registry, addr *net.UDPAddr, every time.Duration) {
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("discovery: beacon dial: %v", err)
		return
	}
	defer conn.Close()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			payload, err := json.Marshal(reg.Self())
			if err == nil {
				_, _ = conn.Write(payload)
			}
		}
	}
}

func listen(ctx context.Context, reg *Registry, addr *net.UDPAddr) {
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("discovery: listen: %v (LAN auto-discovery disabled)", err)
		return
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(1 << 20)
	buf := make([]byte, 4096)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
		var info node.Info
		if err := json.Unmarshal(buf[:n], &info); err == nil {
			reg.upsert(info, "lan")
		}
	}
}

// RunSeeds periodically polls explicit seed control addresses over HTTP so that
// peers unreachable by multicast (e.g. Tailscale) still join the mesh.
func RunSeeds(ctx context.Context, reg *Registry, seeds []string, every time.Duration) {
	if len(seeds) == 0 {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, s := range seeds {
				info, err := fetchSelf(client, s)
				if err == nil {
					reg.upsert(info, "seed")
				}
			}
		}
	}
}

func fetchSelf(c *http.Client, hostport string) (node.Info, error) {
	var info node.Info
	req, err := http.NewRequest("GET", "http://"+hostport+"/api/self", nil)
	if err != nil {
		return info, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}
