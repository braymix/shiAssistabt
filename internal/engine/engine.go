// Package engine provisions the prima.cpp inference binaries on demand, so a
// plain shikA download ("one exe / one apk") can fetch the right engine for its
// platform the first time the user presses Start — no compiler, no separate
// install. Bundles are built by shikA's own release CI and attached to its
// GitHub releases as shika-engine-<platform>.zip.
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

// Repo is the GitHub repository whose releases carry the engine bundles.
const Repo = "braymix/shiAssistabt"

// Platform is the os-arch key used in bundle asset names, e.g. "windows-amd64".
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

// AssetName is the release asset expected for this platform.
func AssetName() string { return "shika-engine-" + Platform() + ".zip" }

// Installed reports whether both engine binaries already exist in dir.
func Installed(dir string) bool {
	return isFile(filepath.Join(dir, ServerName())) && isFile(filepath.Join(dir, CliName()))
}

// LatestAssetURL asks the GitHub API for the newest release and returns the
// download URL of this platform's engine bundle. resolver is injectable for
// tests; pass nil for the real GitHub API.
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
	want := AssetName()
	for _, a := range rel.Assets {
		if a.Name == want {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("no engine bundle %q in the latest release (%s) — this platform may not be built yet", want, rel.TagName)
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
	if !Installed(dir) {
		return fmt.Errorf("engine bundle did not contain %s and %s", ServerName(), CliName())
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
