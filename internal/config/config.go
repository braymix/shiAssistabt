// Package config loads and holds shikA runtime configuration.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config is the full node configuration. It is intentionally small: most of
// the interesting behaviour (who becomes head, the ring order) is decided at
// runtime by the planner, not hard-coded here.
type Config struct {
	// NodeName is a friendly name for this device (defaults to hostname).
	NodeName string `json:"node_name"`

	// Model is the GGUF filename every node must have under PrimaDir/download.
	Model string `json:"model"`

	// PrimaDir is where prima.cpp is checked out and built on this device. It is
	// the working directory the engine binaries run from.
	PrimaDir string `json:"prima_dir"`

	// ServerBin / CliBin are the executables the head and workers launch,
	// relative to PrimaDir (or absolute). Defaults suit a desktop prima.cpp
	// build; on Android the app points these at the bundled native libraries in
	// its nativeLibraryDir (e.g. "./libllama-server.so").
	ServerBin string `json:"server_bin"`
	CliBin    string `json:"cli_bin"`

	// RpcBin is llama.cpp's rpc-server, run on worker nodes so the head can
	// offload model layers to them (the "shared power" across devices). RpcPort
	// is the port each worker's rpc-server listens on.
	RpcBin  string `json:"rpc_bin"`
	RpcPort int    `json:"rpc_port"`

	// ModelDir is where GGUF files live. Relative paths are resolved against
	// PrimaDir (prima.cpp's own layout); an absolute path is used as-is, which
	// Android needs since it must store models in writable app storage rather
	// than next to the read-only bundled binaries.
	ModelDir string `json:"model_dir"`

	// APIAddr is where the local control API + dashboard is served.
	APIAddr string `json:"api_addr"`

	// LLMPort is the OpenAI-compatible port prima.cpp's llama-server binds
	// on the head node (what Open WebUI connects to).
	LLMPort int `json:"llm_port"`

	// Discovery settings for LAN auto-join.
	MulticastAddr string   `json:"multicast_addr"`
	BeaconEvery   Duration `json:"beacon_every"`
	PeerTimeout   Duration `json:"peer_timeout"`

	// Seeds are explicit peer host:port control addresses to contact directly.
	// Use this for Tailscale / cross-subnet peers where multicast does not reach.
	Seeds []string `json:"seeds"`

	// Tailscale, when true, auto-detects a local tailnet and adds its online
	// peers as seeds (additive; LAN multicast still runs). On by default —
	// it is a no-op when tailscale isn't installed or running.
	Tailscale bool `json:"tailscale"`

	// PreferTailscaleIP advertises this node's tailnet IP as its control /
	// prima.cpp address, for clusters whose devices aren't all on one LAN.
	// Off by default so pure-LAN setups keep using their LAN IP.
	PreferTailscaleIP bool `json:"prefer_tailscale_ip"`

	// PrimaCPP ports (data/signal) used between prima.cpp ranks.
	DataPort   int `json:"data_port"`
	SignalPort int `json:"signal_port"`

	// AutoStart, when true, lets shikad launch prima.cpp automatically once a
	// stable plan is reached. Off by default so nothing runs unexpectedly.
	AutoStart bool `json:"auto_start"`
}

// Default returns a config with sensible defaults filled in.
func Default() Config {
	host, _ := os.Hostname()
	if host == "" {
		host = "device"
	}
	return Config{
		NodeName:          host,
		Model:             "qwen2.5-3b-instruct-q4_k_m.gguf",
		PrimaDir:          defaultPrimaDir(),
		ServerBin:         "./llama-server",
		CliBin:            "./llama-cli",
		RpcBin:            "./rpc-server",
		RpcPort:           50052,
		ModelDir:          "download",
		APIAddr:           "0.0.0.0:8977",
		LLMPort:           8080,
		MulticastAddr:     "239.42.42.42:9977",
		BeaconEvery:       Duration(2 * time.Second),
		PeerTimeout:       Duration(10 * time.Second),
		Seeds:             []string{},
		Tailscale:         true,
		PreferTailscaleIP: false,
		DataPort:          9000,
		SignalPort:        10000,
		AutoStart:         false,
	}
}

// ResolvedModelDir returns the absolute directory GGUF files live in: ModelDir
// as-is when absolute, otherwise joined onto PrimaDir. Both the download manager
// and the launched engine must agree on this location.
func (c Config) ResolvedModelDir() string {
	if filepath.IsAbs(c.ModelDir) {
		return c.ModelDir
	}
	return filepath.Join(c.PrimaDir, c.ModelDir)
}

func defaultPrimaDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "prima.cpp"
	}
	return home + "/prima.cpp"
}

// Load reads a JSON config file, overlaying it on top of the defaults. A
// missing file is not an error: defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Duration is a time.Duration that (de)serialises from a human string like "2s".
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// also accept a raw number of nanoseconds
		var n int64
		if err2 := json.Unmarshal(b, &n); err2 == nil {
			*d = Duration(n)
			return nil
		}
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// Std returns the underlying time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }
