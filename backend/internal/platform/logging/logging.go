// Package logging builds the process-wide *slog.Logger. Request- and job-scoped
// loggers derived from it travel in a context.Context via package logctx.
package logging

import (
	"io"
	"log/slog"
)

// Format selects how log records are encoded.
type Format int

const (
	// Text is human-readable key=value output, for local development.
	Text Format = iota
	// JSON is one JSON object per line, for log search in production (Render).
	JSON
)

// Options configures New. The zero value logs Text at Info level and adds no
// service attributes; Writer must be set.
type Options struct {
	// Writer receives the encoded records, usually os.Stdout. New does not close it.
	Writer io.Writer
	Format Format
	Level  slog.Level
	// Service, Env and Version are attached to every record when non-empty, so logs
	// from different deployments and releases can be told apart.
	Service string
	Env     string
	Version string
}

// New returns a logger configured by opts.
func New(opts Options) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{Level: opts.Level}
	var h slog.Handler
	switch opts.Format {
	case JSON:
		h = slog.NewJSONHandler(opts.Writer, handlerOpts)
	case Text:
		h = slog.NewTextHandler(opts.Writer, handlerOpts)
	}
	var attrs []any
	for _, a := range []struct{ key, value string }{
		{"service", opts.Service}, {"env", opts.Env}, {"version", opts.Version},
	} {
		if a.value != "" {
			attrs = append(attrs, slog.String(a.key, a.value))
		}
	}
	return slog.New(h).With(attrs...)
}
