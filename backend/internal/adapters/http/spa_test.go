package http_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	qhttp "github.com/boinkkitty/quizette/backend/internal/adapters/http"
)

const indexHTML = `<!doctype html><div id="root"></div>`

// newSPAServer serves the router with a fake frontend build.
func newSPAServer(t *testing.T) *httptest.Server {
	t.Helper()
	h, err := qhttp.NewRouter(qhttp.Config{
		Logger: slog.New(slog.DiscardHandler),
		Web: fstest.MapFS{
			"index.html":         {Data: []byte(indexHTML)},
			"favicon.svg":        {Data: []byte("<svg/>")},
			"assets/app-1a2b.js": {Data: []byte("console.log(1)")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// response is what a test checks of an HTTP response.
type response struct {
	status int
	header http.Header
	body   string
}

func get(t *testing.T, method, url string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

func TestSPA(t *testing.T) {
	t.Parallel()
	srv := newSPAServer(t)

	tests := []struct {
		name, method, path string
		status             int
		body               string // substring; empty skips the check
		contentType        string // prefix; empty skips the check
		cacheControl       string // empty skips the check
	}{
		{"root serves index", http.MethodGet, "/", 200, indexHTML, "text/html", "no-cache"},
		{"client route serves index", http.MethodGet, "/chapters/123/questions", 200, indexHTML, "text/html", "no-cache"},
		{"index.html serves index", http.MethodGet, "/index.html", 200, indexHTML, "text/html", "no-cache"},
		{"hashed asset is immutable", http.MethodGet, "/assets/app-1a2b.js", 200, "console.log(1)", "text/javascript", "public, max-age=31536000, immutable"},
		{"other file is revalidated", http.MethodGet, "/favicon.svg", 200, "<svg/>", "image/svg+xml", "no-cache"},
		{"missing asset is 404, not index", http.MethodGet, "/assets/app-old.js", 404, "", "", ""},
		{"missing file is 404, not index", http.MethodGet, "/robots.txt", 404, "", "", ""},
		{"HEAD works", http.MethodHead, "/quizzes", 200, "", "text/html", "no-cache"},
		{"unknown API path stays JSON 404", http.MethodGet, "/api/nope", 404, `"not_found"`, "application/json", ""},
		{"health check still answers", http.MethodGet, "/healthz", 200, `"ok"`, "application/json", ""},
		{"POST to a page is rejected", http.MethodPost, "/quizzes", 405, "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := get(t, tc.method, srv.URL+tc.path)
			if resp.status != tc.status {
				t.Fatalf("status = %d, want %d (body %q)", resp.status, tc.status, resp.body)
			}
			if !strings.Contains(resp.body, tc.body) {
				t.Errorf("body = %q, want it to contain %q", resp.body, tc.body)
			}
			if ct := resp.header.Get("Content-Type"); !strings.HasPrefix(ct, tc.contentType) {
				t.Errorf("Content-Type = %q, want prefix %q", ct, tc.contentType)
			}
			if cc := resp.header.Get("Cache-Control"); tc.cacheControl != "" && cc != tc.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", cc, tc.cacheControl)
			}
		})
	}
}

func TestNoFrontendServesAPIOnly(t *testing.T) {
	t.Parallel()
	h, err := qhttp.NewRouter(qhttp.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	if resp := get(t, http.MethodGet, srv.URL+"/"); resp.status != http.StatusNotFound {
		t.Errorf("GET / = %d, want 404 without a frontend", resp.status)
	}
}

func TestFrontendWithoutIndexIsAnError(t *testing.T) {
	t.Parallel()
	_, err := qhttp.NewRouter(qhttp.Config{
		Logger: slog.New(slog.DiscardHandler),
		Web:    fstest.MapFS{"assets/app.js": {Data: []byte("x")}},
	})
	if err == nil {
		t.Fatal("NewRouter accepted a frontend without index.html")
	}
}
