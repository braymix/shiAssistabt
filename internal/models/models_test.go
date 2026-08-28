package models

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFitsMesh(t *testing.T) {
	m := Model{MinRAMGB: 8}
	if m.FitsMesh(7.9) {
		t.Fatal("7.9GB mesh should not fit an 8GB model")
	}
	if !m.FitsMesh(8) {
		t.Fatal("exactly 8GB should fit an 8GB model")
	}
	if !m.FitsMesh(32) {
		t.Fatal("32GB mesh should fit an 8GB model")
	}
}

func TestCatalogIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Catalog() {
		if m.ID == "" || m.File == "" || m.URL == "" {
			t.Fatalf("incomplete catalog entry: %+v", m)
		}
		if seen[m.ID] {
			t.Fatalf("duplicate catalog id %q", m.ID)
		}
		seen[m.ID] = true
	}
	if _, ok := Find("qwen2.5-3b-instruct-q4km"); !ok {
		t.Fatal("default model missing from catalog")
	}
}

func TestInstalledDetectsFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	m := Model{ID: "x", File: "x.gguf"}
	if mgr.Installed(m) {
		t.Fatal("should not be installed before download")
	}
	if err := os.MkdirAll(mgr.DownloadDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mgr.DownloadDir(), m.File), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mgr.Installed(m) {
		t.Fatal("should detect the written file")
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	payload := []byte("pretend this is a gguf file")
	sum := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewManager(dir)

	// Correct hash -> installs.
	good := Model{ID: "good", File: "good.gguf", URL: srv.URL, SHA256: hex.EncodeToString(sum[:])}
	mgr.Start(good)
	waitDone(t, mgr, "good")
	if !mgr.Installed(good) {
		t.Fatal("verified model should be installed")
	}
	if p, _ := mgr.Progress("good"); p.Error != "" || !p.Completed {
		t.Fatalf("progress = %+v, want completed with no error", p)
	}

	// Wrong hash -> rejected, no file left behind.
	bad := Model{ID: "bad", File: "bad.gguf", URL: srv.URL, SHA256: "deadbeef"}
	mgr.Start(bad)
	waitDone(t, mgr, "bad")
	if mgr.Installed(bad) {
		t.Fatal("checksum-mismatched file must not be installed")
	}
	if p, _ := mgr.Progress("bad"); p.Error == "" {
		t.Fatal("expected a checksum error")
	}
	if _, err := os.Stat(filepath.Join(mgr.DownloadDir(), "bad.gguf.part")); !os.IsNotExist(err) {
		t.Fatal("partial file should be cleaned up")
	}
}

func waitDone(t *testing.T, mgr *Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p, ok := mgr.Progress(id); ok && !p.Active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("download %q did not finish", id)
}
