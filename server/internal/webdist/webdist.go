// Package webdist embeds the built couple web app so the standalone server can
// answer http://host:port/ without extra files next to the executable.
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var content embed.FS

// Dist returns the embedded single-page application, or nil when the build only
// contains the placeholder (a Go-only build without `npm run build`).
func Dist() fs.FS {
	sub, err := fs.Sub(content, "dist")
	if err != nil {
		return nil
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.Name() != ".keep" {
			return sub
		}
	}
	return nil
}