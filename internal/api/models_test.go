package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/models"
	"github.com/braymix/shika/internal/node"
	"github.com/braymix/shika/internal/supervisor"
)

func TestHandleModelSelectAdvertisesModel(t *testing.T) {
	self := node.Info{ID: "self", Control: "127.0.0.1:8977", LLMPort: 8080, RAMBytes: 32 << 30, Cores: 8}
	reg := discovery.NewRegistry(self, 10*time.Second)
	srv := New(config.Default(), reg, supervisor.New(t.TempDir(), self.ID))

	// Pick a real catalog model and select it.
	target := models.Catalog()[0]
	rec := httptest.NewRecorder()
	srv.handleModelSelect(rec, httptest.NewRequest(http.MethodPost, "/api/models/select?id="+target.ID, nil))
	if rec.Result().StatusCode != http.StatusOK {
		body, _ := io.ReadAll(rec.Result().Body)
		t.Fatalf("select failed: %s", body)
	}

	// The node now advertises that model file, so the plan it builds runs it.
	if reg.Self().Model != target.File {
		t.Fatalf("self model = %q, want %q", reg.Self().Model, target.File)
	}
	plan, ok := srv.Plan()
	if !ok || plan.Model != target.File {
		t.Fatalf("plan model = %q (ok=%v), want %q", plan.Model, ok, target.File)
	}
}

func TestHandleModelSelectUnknown(t *testing.T) {
	self := node.Info{ID: "self", Control: "127.0.0.1:8977", LLMPort: 8080, RAMBytes: 8 << 30, Cores: 4}
	reg := discovery.NewRegistry(self, 10*time.Second)
	srv := New(config.Default(), reg, supervisor.New(t.TempDir(), self.ID))

	rec := httptest.NewRecorder()
	srv.handleModelSelect(rec, httptest.NewRequest(http.MethodPost, "/api/models/select?id=nope", nil))
	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for unknown model", rec.Result().StatusCode)
	}
}
