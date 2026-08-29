// Package engine provisions the llama.cpp inference binaries on demand, so a
// plain shikA download ("one exe / one apk") can fetch the right engine for its
// platform the first time the user presses Start — no compiler, no separate
// install. It pulls the official prebuilt binaries from llama.cpp's GitHub
// releases (the CPU build for the platform), which is reliable and needs no
// build step of our own.
package engine

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Repo is the GitHub repository whose releases carry the prebuilt engine.
const Repo = "ggml-org/llama.cpp"

// Platform is the os-arch key used for diagnostics, e.g. "windows-amd64".
func Platform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// ServerName / CliName are the engine executables shikA launches (the head runs
// the server, workers the cli), with the platform's executable extension.
func ServerName() string { return "llama-server" + exeExt() }
func CliName() string    { return "llama-cli" + exeExt() }

// Installed reports whether the engine binaries already exist in dir. The RPC
// worker binary is bundled by llama.cpp too but the head only strictly needs the
// server; we key "installed" on the server binary being present.
func Installed(dir string) bool {
	return isFile(filepath.Join(dir, ServerName()))
}

// osToken / archToken identify this platform inside llama.cpp asset names
// (e.g. "llama-b6600-bin-win-cpu-x64.zip").
func osToken() string {
	switch runtime.GOOS {
	case "windows":
		return "win"
	case "darwin":
		return "macos"
	case "linux":
		return "ubuntu"
	}
	return runtime.GOOS
}

func archToken() string {
	if runtime.GOARCH == "amd64" {
		return "x64"
	}
	return runtime.GOARCH // arm64
}

// gpuTokens name accelerator builds we avoid (they need drivers we can't assume).
var gpuTokens = []string{"cuda", "vulkan", "hip", "sycl", "kompute", "musa", "cann"}

// LatestAssetURL asks the GitHub API for llama.cpp's newest release and returns
// the download URL of the plain CPU build for this platform. resolver is
// injectable for tests; pass nil for the real GitHub API.
func LatestAssetURL(ctx context.Context, resolver func(context.Context) ([]byte, error)) (string, error) {
	if resolver == nil {
		resolver = fetchLatestRelease
	}
	data, err := resolver(ctx)
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &rel); err != nil {
		return "", err
	}

	oss, arch := osToken(), archToken()
	for _, a := range rel.Assets {
		n := strings.ToLower(a.Name)
		if !strings.HasSuffix(n, ".zip") || !strings.Contains(n, "bin-") {
			continue
		}
		if !strings.Contains(n, oss) || !strings.Contains(n, arch) {
			continue
		}
		if containsAny(n, gpuTokens) {
			continue // prefer the plain CPU build
		}
		return a.URL, nil
	}
	return "", fmt.Errorf("no CPU llama.cpp build for %s in release %s", Platform(), rel.TagName)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func fetchLatestRelease(ctx context.Context) ([]byte, error) {
	url := "https://api.github.com/repos/" + Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases API: HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Ensure makes sure the engine is present in dir, downloading and extracting the
// bundle at url if it is not. progress (optional) reports bytes downloaded.
func Ensure(ctx context.Context, dir, url string, progress func(done, total int64)) error {
	if Installed(dir) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "engine-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		tmp.Close()
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmp.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		return fmt.Errorf("engine download: HTTP %s", resp.Status)
	}

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := unzip(tmpName, dir); err != nil {
		return err
	}
	// llama.cpp zips sometimes nest the binaries under a subfolder (e.g.
	// build/bin/). Flatten so the launcher finds ./llama-server next to its libs.
	if !Installed(dir) {
		if err := flatten(dir); err != nil {
			return err
		}
	}
	if !Installed(dir) {
		return fmt.Errorf("engine archive did not contain %s", ServerName())
	}
	return nil
}

// flatten locates ServerName anywhere under dir and, if it sits in a subfolder,
// moves every file from that folder up to dir so the binary and its shared
// libraries land together at the top level.
func flatten(dir string) error {
	var found string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || found != "" || fi.IsDir() {
			return nil
		}
		if filepath.Base(p) == ServerName() {
			found = p
		}
		return nil
	})
	if found == "" {
		return nil
	}
	srcDir := filepath.Dir(found)
	if srcDir == filepath.Clean(dir) {
		return nil
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(srcDir, e.Name())
		to := filepath.Join(dir, e.Name())
		if e.IsDir() {
			continue // engine binaries and libs are files
		}
		_ = os.Remove(to)
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}

// unzip extracts a zip archive into dir, flattening nothing but keeping the
// archive's own layout, and marks the engine binaries executable.
func unzip(zipPath, dir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Guard against zip-slip: never let an entry escape dir.
		dest := filepath.Join(dir, f.Name)
		if !strings.HasPrefix(dest, filepath.Clean(dir)+string(os.PathSeparator)) && dest != filepath.Clean(dir) {
			return fmt.Errorf("unsafe path in archive: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := f.Mode()
		if base := filepath.Base(f.Name); base == ServerName() || base == CliName() {
			mode |= 0o755
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
