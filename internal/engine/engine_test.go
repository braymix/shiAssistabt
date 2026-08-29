package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// makeBundle builds an in-memory zip containing the two engine binaries plus a
// stray support file, mimicking a real engine bundle.
func makeBundle(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{ServerName(), CliName(), "libggml.so"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("binary:" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEnsureDownloadsAndExtracts(t *testing.T) {
	bundle := makeBundle(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bundle)
	}))
	defer srv.Close()

	dir := t.TempDir()
	if Installed(dir) {
		t.Fatal("should not be installed before Ensure")
	}
	if err := Ensure(context.Background(), dir, srv.URL, nil); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !Installed(dir) {
		t.Fatal("engine should be installed after Ensure")
	}
	// The server binary must be present and (on unix) executable.
	fi, err := os.Stat(filepath.Join(dir, ServerName()))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o100 == 0 {
		t.Fatalf("server binary not executable: mode %v", fi.Mode())
	}
	// A second Ensure is a no-op (already installed) — must not error.
	if err := Ensure(context.Background(), dir, "http://invalid.invalid/should-not-be-hit", nil); err != nil {
		t.Fatalf("second Ensure should no-op, got %v", err)
	}
}

// releaseJSON mimics llama.cpp's release assets for every platform, so the
// resolver must pick the plain CPU build matching THIS runner's os/arch.
const releaseJSON = `{
  "tag_name": "b6600",
  "assets": [
    {"name": "llama-b6600-bin-win-cpu-x64.zip",        "browser_download_url": "http://x/win-cpu-x64"},
    {"name": "llama-b6600-bin-win-cuda-12.4-x64.zip",  "browser_download_url": "http://x/win-cuda"},
    {"name": "llama-b6600-bin-macos-arm64.zip",        "browser_download_url": "http://x/macos-arm64"},
    {"name": "llama-b6600-bin-macos-x64.zip",          "browser_download_url": "http://x/macos-x64"},
    {"name": "llama-b6600-bin-ubuntu-x64.zip",         "browser_download_url": "http://x/ubuntu-x64"},
    {"name": "llama-b6600-bin-ubuntu-vulkan-x64.zip",  "browser_download_url": "http://x/ubuntu-vulkan"}
  ]
}`

func TestLatestAssetURLPicksPlatformCPUBuild(t *testing.T) {
	url, err := LatestAssetURL(context.Background(), func(context.Context) ([]byte, error) {
		return []byte(releaseJSON), nil
	})
	// Only assert on the platforms the CI actually runs (linux/darwin); the
	// resolver returns an error for anything not in the fixture.
	switch osToken() + "-" + archToken() {
	case "ubuntu-x64":
		if err != nil || url != "http://x/ubuntu-x64" {
			t.Fatalf("linux amd64: url=%q err=%v, want the CPU ubuntu x64 build", url, err)
		}
	case "macos-arm64":
		if err != nil || url != "http://x/macos-arm64" {
			t.Fatalf("darwin arm64: url=%q err=%v", url, err)
		}
	case "macos-x64":
		if err != nil || url != "http://x/macos-x64" {
			t.Fatalf("darwin amd64: url=%q err=%v", url, err)
		}
	default:
		// Other platforms may legitimately have no fixture match.
	}
	// Whatever platform, it must never pick a GPU build.
	if err == nil && (contains(url, "cuda") || contains(url, "vulkan")) {
		t.Fatalf("picked a GPU build: %q", url)
	}
}

func TestLatestAssetURLNoMatch(t *testing.T) {
	body := `{"tag_name":"b1","assets":[{"name":"llama-b1-bin-freebsd-x64.zip","browser_download_url":"http://x/bsd"}]}`
	if _, err := LatestAssetURL(context.Background(), func(context.Context) ([]byte, error) {
		return []byte(body), nil
	}); err == nil {
		t.Fatal("expected an error when no asset matches this platform")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
