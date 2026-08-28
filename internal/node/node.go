// Package node describes this device's identity and hardware capabilities.
package node

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Info is the self-description a node advertises to the mesh. It is small and
// JSON-friendly so it can be gossiped in a single UDP beacon.
type Info struct {
	ID       string `json:"id"`        // stable random id for this process/device
	Name     string `json:"name"`      // friendly name
	OS       string `json:"os"`        // linux, darwin, windows, android...
	Arch     string `json:"arch"`      // arm64, amd64...
	Cores    int    `json:"cores"`     // logical CPUs
	RAMBytes uint64 `json:"ram_bytes"` // total physical RAM
	HasGPU   bool   `json:"has_gpu"`   // best-effort GPU presence hint
	Control  string `json:"control"`   // host:port of this node's control API
	LLMPort  int    `json:"llm_port"`  // OpenAI port used if this node is head
}

// RAMGB is a convenience for display.
func (i Info) RAMGB() float64 { return float64(i.RAMBytes) / (1024 * 1024 * 1024) }

// Capability is a single scalar used to rank nodes for the "head" role.
// More RAM dominates; cores and GPU act as tie-breakers. The head holds the
// output layer and runs the HiGHS planner, so the strongest device should win.
func (i Info) Capability() float64 {
	score := i.RAMGB()*10 + float64(i.Cores)
	if i.HasGPU {
		score += 5
	}
	return score
}

// Detect builds an Info for the current device.
func Detect(name, control string, llmPort int) Info {
	if name == "" {
		name, _ = os.Hostname()
	}
	return Info{
		ID:       ephemeralID(),
		Name:     name,
		OS:       detectOS(),
		Arch:     runtime.GOARCH,
		Cores:    runtime.NumCPU(),
		RAMBytes: totalRAM(),
		HasGPU:   detectGPU(),
		Control:  control,
		LLMPort:  llmPort,
	}
}

func ephemeralID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "node-unknown"
	}
	return hex.EncodeToString(b)
}

// detectOS returns runtime.GOOS but upgrades linux->android when running under
// Termux, since that changes how prima.cpp is launched.
func detectOS() string {
	if runtime.GOOS == "linux" {
		if _, ok := os.LookupEnv("TERMUX_VERSION"); ok {
			return "android"
		}
		if _, err := os.Stat("/data/data/com.termux/files/usr"); err == nil {
			return "android"
		}
	}
	return runtime.GOOS
}

// totalRAM returns total physical memory in bytes, best-effort per platform.
func totalRAM() uint64 {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return v
			}
		}
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							return kb * 1024
						}
					}
				}
			}
		}
	}
	return 0
}

// detectGPU is a cheap best-effort hint, not authoritative.
func detectGPU() bool {
	switch runtime.GOOS {
	case "darwin":
		// Apple Silicon always has a usable Metal GPU.
		return runtime.GOARCH == "arm64"
	case "linux":
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			return true
		}
		if _, err := os.Stat("/dev/nvidia0"); err == nil {
			return true
		}
	}
	return false
}
