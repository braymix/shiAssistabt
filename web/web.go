// Package web embeds the device-management dashboard static assets so the
// orchestrator ships as a single self-contained binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dashboard
var content embed.FS

// FS returns the dashboard file system rooted at the dashboard/ directory.
func FS() fs.FS {
	sub, err := fs.Sub(content, "dashboard")
	if err != nil {
		panic(err) // embedded path is compile-time constant; cannot fail
	}
	return sub
}
