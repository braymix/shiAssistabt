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

func TestLatestAssetURLPicksPlatform(t *testing.T) {
	want := AssetName()
	body := `{
      "tag_name": "v0.2.0",
      "assets": [
        {"name": "other.zip", "browser_download_url": "http://x/other.zip"},
        {"name": "` + want + `", "browser_download_url": "http://x/` + want + `"}
      ]
    }`
	url, err := LatestAssetURL(context.Background(), func(context.Context) ([]byte, error) {
		return []byte(body), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://x/"+want {
		t.Fatalf("url = %q, want the platform asset", url)
	}
}

func TestLatestAssetURLMissingPlatform(t *testing.T) {
	body := `{"tag_name":"v0.2.0","assets":[{"name":"nope.zip","browser_download_url":"http://x/nope"}]}`
	_, err := LatestAssetURL(context.Background(), func(context.Context) ([]byte, error) {
		return []byte(body), nil
	})
	if err == nil {
		t.Fatal("expected an error when this platform's bundle is absent")
	}
}
