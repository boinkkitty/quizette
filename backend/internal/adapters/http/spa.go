package http

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// spa serves the built single-page app: files that exist are served as-is,
// and any other path without a file extension gets index.html, so the
// client-side router can handle it.
//
// Vite puts content-hashed files under assets/, so those are cached for a
// year. index.html and the other files are revalidated on every load, so a
// deploy reaches browsers at once.
type spa struct {
	files fs.FS
	index []byte
}

func newSPA(files fs.FS) (*spa, error) {
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		return nil, err //nolint:wrapcheck // caller adds context
	}
	return &spa{files: files, index: index}, nil
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" && name != "index.html" {
		if info, err := fs.Stat(s.files, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			// fs.FS paths cannot leave the embedded build: fs.Stat above rejects "..".
			http.ServeFileFS(w, r, s.files, name) //nolint:gosec // see above
			return
		}
		// A missing file (such as a stale hashed asset after a deploy) must not
		// get index.html: the browser would try to run HTML as a script.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(s.index))
}
