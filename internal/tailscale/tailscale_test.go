package tailscale

import (
	"errors"
	"testing"
)

const sample = `{
  "Self": {
    "HostName": "macbook",
    "DNSName": "macbook.tailnet-abc.ts.net.",
    "TailscaleIPs": ["100.64.0.1", "fd7a:115c:a1e0::1"],
    "Online": true
  },
  "Peer": {
    "nodekey:aaa": {
      "HostName": "phone",
      "TailscaleIPs": ["100.64.0.2", "fd7a:115c:a1e0::2"],
      "Online": true
    },
    "nodekey:bbb": {
      "HostName": "offline-pc",
      "TailscaleIPs": ["100.64.0.3"],
      "Online": false
    }
  }
}`

func TestParsePicksSelfAndOnlinePeers(t *testing.T) {
	info, ok := Parse([]byte(sample))
	if !ok {
		t.Fatal("expected usable info")
	}
	if info.SelfIP != "100.64.0.1" {
		t.Fatalf("self ip = %q, want the v4 tailnet address", info.SelfIP)
	}
	if len(info.Peers) != 1 {
		t.Fatalf("got %d peers, want 1 (offline excluded)", len(info.Peers))
	}
	if info.Peers[0].HostName != "phone" || info.Peers[0].IP != "100.64.0.2" {
		t.Fatalf("peer = %+v", info.Peers[0])
	}
}

func TestSeedsUseControlPort(t *testing.T) {
	info, _ := Parse([]byte(sample))
	seeds := Seeds(info, 8977)
	if len(seeds) != 1 || seeds[0] != "100.64.0.2:8977" {
		t.Fatalf("seeds = %v, want [100.64.0.2:8977]", seeds)
	}
}

func TestDetectHandlesMissingTailscale(t *testing.T) {
	// Runner error (binary absent / daemon down) must degrade to not-available.
	if _, ok := Detect(func() ([]byte, error) { return nil, errors.New("not found") }); ok {
		t.Fatal("expected ok=false when tailscale is unavailable")
	}
	// Empty tailnet (no self, no peers) is likewise not useful.
	if _, ok := Detect(func() ([]byte, error) { return []byte(`{"Peer":{}}`), nil }); ok {
		t.Fatal("expected ok=false for an empty tailnet")
	}
}

func TestParseGarbage(t *testing.T) {
	if _, ok := Parse([]byte("not json")); ok {
		t.Fatal("garbage input should not parse")
	}
}
