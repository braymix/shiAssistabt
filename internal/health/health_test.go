package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeLLMHealthy(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Feed a /v1 base URL like the plan produces; the probe must strip it.
	res := ProbeLLM(context.Background(), srv.URL+"/v1", time.Second)
	if !res.OK {
		t.Fatalf("expected healthy, got %+v", res)
	}
	if gotPath != "/health" {
		t.Fatalf("probed %q, want /health", gotPath)
	}
}

func TestProbeLLMLoading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	res := ProbeLLM(context.Background(), srv.URL, time.Second)
	if res.OK {
		t.Fatal("503 should not be healthy")
	}
	if res.Detail != "loading model" {
		t.Fatalf("detail = %q, want loading model", res.Detail)
	}
}

func TestProbeLLMRefused(t *testing.T) {
	// Nothing is listening on this port.
	res := ProbeLLM(context.Background(), "http://127.0.0.1:1/v1", 500*time.Millisecond)
	if res.OK {
		t.Fatal("expected unhealthy for closed port")
	}
	if res.Detail == "" {
		t.Fatal("expected a failure detail")
	}
}
