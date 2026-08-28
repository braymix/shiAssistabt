package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
	"github.com/braymix/shika/internal/supervisor"
)

// stateServer returns an httptest server whose /api/state reports the given
// running flag, plus its host:port control address.
func stateServer(t *testing.T, running bool) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"state": supervisor.State{Running: running},
		})
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return srv, u.Host
}

func TestPeerRunning(t *testing.T) {
	_, upHost := stateServer(t, true)
	if !peerRunning(upHost) {
		t.Fatal("peer reporting running=true should be considered running")
	}

	_, downHost := stateServer(t, false)
	if peerRunning(downHost) {
		t.Fatal("peer reporting running=false should not be considered running")
	}

	if peerRunning("127.0.0.1:1") {
		t.Fatal("unreachable peer should not be considered running")
	}
	if peerRunning("") {
		t.Fatal("empty control address should not be considered running")
	}
}

func TestWorkersReadySingleNodeHead(t *testing.T) {
	self := node.Info{ID: "self", Name: "solo", Control: "127.0.0.1:8977", LLMPort: 8080, RAMBytes: 8 << 30, Cores: 4}
	reg := discovery.NewRegistry(self, 10*time.Second)
	srv := New(config.Default(), reg, supervisor.New(t.TempDir(), self.ID))

	// A lone node is its own head with no workers to wait for, so it is ready.
	if !srv.WorkersReady() {
		t.Fatal("single-node head should be ready with no workers")
	}
}
