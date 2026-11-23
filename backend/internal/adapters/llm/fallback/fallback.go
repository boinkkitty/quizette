// Package fallback chains question generators: when one provider fails with a
// transient error, or runs out of its time budget, the next one is tried.
package fallback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

// Link is one generator in a chain.
type Link struct {
	// Name identifies the provider in logs, e.g. "gemini".
	Name      string
	Generator app.QuestionGenerator
	// Timeout bounds this link's attempt so later links still have time to
	// run within the caller's deadline. Zero means no limit of its own. The
	// last link always runs with the caller's remaining time.
	Timeout time.Duration
}

// Generator tries each Link in order. It moves on when a link's error wraps
// app.ErrLLMUnavailable or when the link exceeds its own Timeout; any other
// error, including a validation error, is returned at once. It is safe for
// concurrent use if every link is.
type Generator struct {
	Links []Link
}

// New returns a Generator over links. At least one link is required.
func New(links ...Link) (*Generator, error) {
	if len(links) == 0 {
		return nil, errors.New("fallback: at least one generator is required")
	}
	return &Generator{Links: links}, nil
}

// Generate implements app.QuestionGenerator. The result and metrics are those
// of the last link tried.
func (g *Generator) Generate(ctx context.Context, in app.GenerateInput) (app.GenerationResult, error) {
	var (
		res app.GenerationResult
		err error
	)
	last := len(g.Links) - 1
	for i, l := range g.Links {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if l.Timeout > 0 && i < last {
			attemptCtx, cancel = context.WithTimeout(ctx, l.Timeout)
		}
		res, err = l.Generator.Generate(attemptCtx, in)
		timedOut := attemptCtx.Err() != nil && ctx.Err() == nil
		cancel()
		if err == nil || ctx.Err() != nil || i == last {
			break
		}
		if !timedOut && !errors.Is(err, app.ErrLLMUnavailable) {
			break
		}
		if timedOut {
			err = fmt.Errorf("%w: %s took longer than %s: %w", app.ErrLLMUnavailable, l.Name, l.Timeout, err)
		}
		logctx.From(ctx, slog.Default()).WarnContext(ctx, "llm.provider_fallback",
			slog.String("provider", l.Name), slog.String("next_provider", g.Links[i+1].Name),
			slog.String("model", res.Metrics.Model), slog.Any("error", err))
	}
	return res, err
}
