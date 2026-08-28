// Package health probes whether a launched prima.cpp endpoint is actually
// answering, so the supervisor can report more than "the process is alive".
package health

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// Result is the outcome of a single probe.
type Result struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// llama-server exposes GET /health, returning 200 once the model is loaded and
// the server is ready to accept completions. We treat any 2xx as healthy.
const healthPath = "/health"

// ProbeLLM checks a prima.cpp llama-server head endpoint. base is the
// OpenAI-compatible base URL shown in the plan (e.g. "http://host:8080/v1");
// the /v1 suffix is trimmed before hitting /health. A non-2xx, a connection
// refusal, or a timeout all count as not-ready, with a short human detail.
func ProbeLLM(ctx context.Context, base string, timeout time.Duration) Result {
	url := strings.TrimSuffix(strings.TrimRight(base, "/"), "/v1") + healthPath

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{OK: false, Detail: "bad url"}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{OK: false, Detail: connError(err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Result{OK: true}
	}
	if resp.StatusCode == http.StatusServiceUnavailable {
		// llama-server returns 503 while the model is still loading.
		return Result{OK: false, Detail: "loading model"}
	}
	return Result{OK: false, Detail: "http " + resp.Status}
}

// connError condenses common dial failures into a short, stable message so the
// dashboard shows something readable rather than a full net stack error.
func connError(err error) string {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return "timeout"
	}
	if strings.Contains(err.Error(), "connection refused") {
		return "connection refused"
	}
	return "unreachable"
}
