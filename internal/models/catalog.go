// Package models is shikA's model manager: a curated catalog of GGUF models,
// plus download-with-verification and a "does it fit this mesh?" check.
package models

// Model is one curated GGUF entry the dashboard can offer for download.
type Model struct {
	ID   string `json:"id"`   // stable slug
	Name string `json:"name"` // human label
	File string `json:"file"` // gguf filename, placed under <prima_dir>/download/
	URL  string `json:"url"`  // direct download URL

	// SHA256 pins the file's hash (hex). When set, a download that doesn't match
	// is rejected and deleted. Empty means "not pinned yet" — the download still
	// works but is reported as unverified so maintainers know to fill it in.
	SHA256 string `json:"sha256,omitempty"`

	SizeBytes  int64   `json:"size_bytes"` // approximate, for display + guidance
	MinRAMGB   float64 `json:"min_ram_gb"` // combined mesh RAM this model wants
	Uncensored bool    `json:"uncensored"` // surfaced honestly in the UI
	Notes      string  `json:"notes,omitempty"`
}

// Verified reports whether this entry carries a pinned hash.
func (m Model) Verified() bool { return m.SHA256 != "" }

// FitsMesh reports whether the combined mesh memory covers this model's needs.
func (m Model) FitsMesh(meshRAMGB float64) bool {
	return meshRAMGB+1e-9 >= m.MinRAMGB
}

const gb = 1 << 30

// Catalog is the built-in curated list. Sizes are approximate; SHA256 values are
// filled in per release as files are pinned. URLs point at stable HuggingFace
// resolve links. This is intentionally short and edited by hand — quality over
// quantity, with clear RAM guidance so users pick something their mesh can run.
func Catalog() []Model {
	return []Model{
		{
			ID:        "qwen2.5-0.5b-instruct-q4km",
			Name:      "Qwen2.5 0.5B Instruct (Q4_K_M)",
			File:      "qwen2.5-0.5b-instruct-q4_k_m.gguf",
			URL:       "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q4_k_m.gguf",
			SizeBytes: 491 * (1 << 20),
			MinRAMGB:  1,
			Notes:     "Tiny — great for smoke-testing a fresh mesh.",
		},
		{
			ID:        "qwen2.5-3b-instruct-q4km",
			Name:      "Qwen2.5 3B Instruct (Q4_K_M)",
			File:      "qwen2.5-3b-instruct-q4_k_m.gguf",
			URL:       "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf",
			SizeBytes: 2 * gb,
			MinRAMGB:  4,
			Notes:     "shikA's default — a solid small assistant.",
		},
		{
			ID:        "qwen2.5-7b-instruct-q4km",
			Name:      "Qwen2.5 7B Instruct (Q4_K_M)",
			File:      "qwen2.5-7b-instruct-q4_k_m.gguf",
			// Qwen's own repo ships this quant split across shards; bartowski's
			// is a single file, which shikA's one-file downloader needs.
			URL:       "https://huggingface.co/bartowski/Qwen2.5-7B-Instruct-GGUF/resolve/main/Qwen2.5-7B-Instruct-Q4_K_M.gguf",
			SizeBytes: 5 * gb,
			MinRAMGB:  8,
			Notes:     "Noticeably stronger; wants a couple of decent devices.",
		},
		{
			ID:        "llama3.1-8b-instruct-q4km",
			Name:      "Llama 3.1 8B Instruct (Q4_K_M)",
			File:      "meta-llama-3.1-8b-instruct-q4_k_m.gguf",
			URL:       "https://huggingface.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf",
			SizeBytes: 5 * gb,
			MinRAMGB:  8,
			Notes:     "General-purpose, widely compatible.",
		},
		{
			ID:         "dolphin3.0-llama3.1-8b-q4km",
			Name:       "Dolphin 3.0 Llama 3.1 8B (Q4_K_M)",
			File:       "dolphin3.0-llama3.1-8b-q4_k_m.gguf",
			URL:        "https://huggingface.co/bartowski/Dolphin3.0-Llama3.1-8B-GGUF/resolve/main/Dolphin3.0-Llama3.1-8B-Q4_K_M.gguf",
			SizeBytes:  5 * gb,
			MinRAMGB:   8,
			Uncensored: true,
			Notes:      "Uncensored fine-tune. You choose what you run on your own hardware.",
		},
	}
}

// Find returns the catalog entry with the given ID.
func Find(id string) (Model, bool) {
	for _, m := range Catalog() {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}
