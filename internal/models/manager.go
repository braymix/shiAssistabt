package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Progress is the live state of a model download, exposed to the dashboard.
type Progress struct {
	ID        string `json:"id"`
	Active    bool   `json:"active"`
	Done      int64  `json:"done"`
	Total     int64  `json:"total"`
	Completed bool   `json:"completed"`
	Error     string `json:"error,omitempty"`
}

// Manager downloads and tracks catalog models under <primaDir>/download.
type Manager struct {
	primaDir string

	mu       sync.Mutex
	progress map[string]*Progress
}

// NewManager creates a model manager rooted at a prima.cpp checkout.
func NewManager(primaDir string) *Manager {
	return &Manager{primaDir: primaDir, progress: make(map[string]*Progress)}
}

// DownloadDir is where GGUF files live (prima.cpp expects them under download/).
func (mgr *Manager) DownloadDir() string {
	return filepath.Join(mgr.primaDir, "download")
}

// Installed reports whether the model's file is present on disk.
func (mgr *Manager) Installed(m Model) bool {
	fi, err := os.Stat(filepath.Join(mgr.DownloadDir(), m.File))
	return err == nil && !fi.IsDir()
}

// Progress returns a snapshot of the download state for one model.
func (mgr *Manager) Progress(id string) (Progress, bool) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	p, ok := mgr.progress[id]
	if !ok {
		return Progress{}, false
	}
	return *p, true
}

// Start kicks off a background download if one isn't already running for this
// model. It returns immediately; poll Progress to follow along.
func (mgr *Manager) Start(m Model) {
	mgr.mu.Lock()
	if p, ok := mgr.progress[m.ID]; ok && p.Active {
		mgr.mu.Unlock()
		return // already downloading
	}
	mgr.progress[m.ID] = &Progress{ID: m.ID, Active: true}
	mgr.mu.Unlock()

	go func() {
		err := mgr.download(context.Background(), m)
		mgr.mu.Lock()
		p := mgr.progress[m.ID]
		p.Active = false
		if err != nil {
			p.Error = err.Error()
		} else {
			p.Completed = true
		}
		mgr.mu.Unlock()
	}()
}

// set updates the tracked byte counters for a model.
func (mgr *Manager) set(id string, done, total int64) {
	mgr.mu.Lock()
	if p, ok := mgr.progress[id]; ok {
		p.Done, p.Total = done, total
	}
	mgr.mu.Unlock()
}

// download streams the model to a temp file, verifies its hash when pinned, and
// atomically moves it into place. Any failure removes the partial file.
func (mgr *Manager) download(ctx context.Context, m Model) error {
	if err := os.MkdirAll(mgr.DownloadDir(), 0o755); err != nil {
		return err
	}
	dest := filepath.Join(mgr.DownloadDir(), m.File)
	tmp := dest + ".part"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %s", resp.Status)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	total := resp.ContentLength
	mgr.set(m.ID, 0, total)

	pr := &progressReader{r: resp.Body, total: total, onProgress: func(done int64) {
		mgr.set(m.ID, done, total)
	}}
	if _, err := io.Copy(io.MultiWriter(f, hasher), pr); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if m.SHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, m.SHA256) {
			os.Remove(tmp)
			return fmt.Errorf("checksum mismatch: got %s, want %s", got, m.SHA256)
		}
	}
	return os.Rename(tmp, dest)
}

// progressReader reports bytes read, throttled so we don't lock on every chunk.
type progressReader struct {
	r          io.Reader
	total      int64
	done       int64
	last       time.Time
	onProgress func(done int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.onProgress != nil && (time.Since(p.last) > 200*time.Millisecond || err != nil) {
		p.last = time.Now()
		p.onProgress(p.done)
	}
	return n, err
}
