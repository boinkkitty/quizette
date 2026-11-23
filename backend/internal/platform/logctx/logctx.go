// Package logctx carries a request- or job-scoped *slog.Logger in a context so
// logs from HTTP handlers, use cases and adapters share correlation IDs.
package logctx

import (
	"context"
	"log/slog"
)

type key struct{}

// With returns a context carrying logger.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, key{}, logger)
}

// From returns the context logger, or fallback when none is set.
// A nil fallback yields a logger that discards everything.
func From(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(key{}).(*slog.Logger); ok && l != nil {
		return l
	}
	if fallback != nil {
		return fallback
	}
	return slog.New(slog.DiscardHandler)
}

// AddAttrs returns a context whose logger has the extra attributes.
func AddAttrs(ctx context.Context, fallback *slog.Logger, attrs ...any) context.Context {
	return With(ctx, From(ctx, fallback).With(attrs...))
}

// Secret is a string that never appears in logs.
type Secret string

// LogValue redacts the secret.
func (Secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
