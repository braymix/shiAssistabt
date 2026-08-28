// Package tailscale detects a local Tailscale node and turns its peer list into
// shikA seeds, so devices on a tailnet (but not the same LAN) still find each
// other without hand-entering IP addresses.
package tailscale

import (
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Info is the distilled view of `tailscale status --json` shikA cares about.
type Info struct {
	SelfIP string // this node's tailnet IPv4, if any
	Peers  []Peer
}

// Peer is one online tailnet peer.
type Peer struct {
	HostName string
	IP       string // tailnet IPv4
}

// raw mirrors the subset of the tailscale status JSON we read.
type raw struct {
	Self *device            `json:"Self"`
	Peer map[string]*device `json:"Peer"`
}

type device struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

// Runner returns the raw bytes of `tailscale status --json`. It is a field so
// tests can inject fixtures without a real tailscale binary.
type Runner func() ([]byte, error)

// defaultRunner shells out to the tailscale CLI, if present.
func defaultRunner() ([]byte, error) {
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, path, "status", "--json").Output()
}

// Detect runs the given Runner (or the default when nil) and parses the result.
// ok is false when Tailscale is absent, not running, or the output is unusable —
// callers should simply carry on with LAN/seed discovery in that case.
func Detect(run Runner) (Info, bool) {
	if run == nil {
		run = defaultRunner
	}
	data, err := run()
	if err != nil || len(data) == 0 {
		return Info{}, false
	}
	return Parse(data)
}

// Parse extracts self and online peers from status JSON.
func Parse(data []byte) (Info, bool) {
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return Info{}, false
	}
	info := Info{}
	if r.Self != nil {
		info.SelfIP = firstV4(r.Self.TailscaleIPs)
	}
	for _, d := range r.Peer {
		if d == nil || !d.Online {
			continue
		}
		ip := firstV4(d.TailscaleIPs)
		if ip == "" {
			continue
		}
		info.Peers = append(info.Peers, Peer{HostName: hostName(d), IP: ip})
	}
	// A tailnet with no self IP and no online peers is not useful to us.
	if info.SelfIP == "" && len(info.Peers) == 0 {
		return info, false
	}
	return info, true
}

// Seeds turns online peers into "ip:port" control addresses for seed discovery,
// assuming every node serves its control API on the same port as this one.
func Seeds(info Info, controlPort int) []string {
	if controlPort <= 0 {
		return nil
	}
	out := make([]string, 0, len(info.Peers))
	for _, p := range info.Peers {
		out = append(out, net.JoinHostPort(p.IP, strconv.Itoa(controlPort)))
	}
	return out
}

func firstV4(ips []string) string {
	for _, ip := range ips {
		if strings.Contains(ip, ".") && !strings.Contains(ip, ":") {
			return ip
		}
	}
	return ""
}

func hostName(d *device) string {
	if d.HostName != "" {
		return d.HostName
	}
	// DNSName is like "phone.tailnet-xxxx.ts.net."; take the first label.
	if i := strings.IndexByte(d.DNSName, '.'); i > 0 {
		return d.DNSName[:i]
	}
	return d.DNSName
}
