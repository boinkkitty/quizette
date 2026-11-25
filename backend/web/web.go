// Package web holds the built frontend, embedded into the API binary.
//
// The Docker build copies frontend/dist into dist/ before compiling. A local
// build has only a placeholder there, so the API serves no frontend and the
// Vite dev server serves it instead.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend, or false when the binary was built without it.
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}
