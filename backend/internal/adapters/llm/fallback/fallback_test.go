package fallback_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/fallback"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// fake is a generator that returns err, or blocks until its context ends when hang is set.
type fake struct {
	name  string
	err   error
	hang  bool
	calls *[]string
	mu    *sync.Mutex
}

func (f fake) Generate(ctx context.Context, _ app.GenerateInput) (app.GenerationResult, error) {
	f.mu.Lock()
	*f.calls = append(*f.calls, f.name)
	f.mu.Unlock()
	res := app.GenerationResult{Metrics: domain.GenerationMetrics{Model: f.name}}
	if f.hang {
		<-ctx.Done()
		return res, fmt.Errorf("%s: %w", f.name, ctx.Err())
	}
	return res, f.err
}

func TestFallback(t *testing.T) {
	t.Parallel()
	unavailable := fmt.Errorf("%w: 503 overloaded", app.ErrLLMUnavailable)
	invalid := domain.Invalid("not JSON")
	tests := []struct {
		name      string
		first     fake
		second    fake
		wantCalls []string
		wantModel string
		wantErr   error
	}{
		{"first succeeds", fake{name: "a"}, fake{name: "b"}, []string{"a"}, "a", nil},
		{"unavailable falls back", fake{name: "a", err: unavailable}, fake{name: "b"}, []string{"a", "b"}, "b", nil},
		{"hang past timeout falls back", fake{name: "a", hang: true}, fake{name: "b"}, []string{"a", "b"}, "b", nil},
		{"validation error does not fall back", fake{name: "a", err: invalid}, fake{name: "b"}, []string{"a"}, "a", invalid},
		{"last error is returned", fake{name: "a", err: unavailable}, fake{name: "b", err: unavailable}, []string{"a", "b"}, "b", app.ErrLLMUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			var mu sync.Mutex
			for _, f := range []*fake{&tt.first, &tt.second} {
				f.calls, f.mu = &calls, &mu
			}
			g, err := fallback.New(
				fallback.Link{Name: "first", Generator: tt.first, Timeout: 50 * time.Millisecond},
				fallback.Link{Name: "second", Generator: tt.second, Timeout: time.Millisecond}, // ignored: last link
			)
			if err != nil {
				t.Fatal(err)
			}
			res, err := g.Generate(context.Background(), app.GenerateInput{})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			if res.Metrics.Model != tt.wantModel {
				t.Errorf("model = %q, want %q", res.Metrics.Model, tt.wantModel)
			}
		})
	}
}

// When the caller's own deadline expires, the chain stops instead of starting
// the next provider with no time left.
func TestCallerDeadlineStopsChain(t *testing.T) {
	t.Parallel()
	var calls []string
	var mu sync.Mutex
	g, _ := fallback.New(
		fallback.Link{Name: "first", Generator: fake{name: "a", hang: true, calls: &calls, mu: &mu}, Timeout: time.Minute},
		fallback.Link{Name: "second", Generator: fake{name: "b", calls: &calls, mu: &mu}},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := g.Generate(ctx, app.GenerateInput{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
	if !slices.Equal(calls, []string{"a"}) {
		t.Errorf("calls = %v, want [a]", calls)
	}
}

func TestNewRequiresLinks(t *testing.T) {
	t.Parallel()
	if _, err := fallback.New(); err == nil {
		t.Error("want error")
	}
}
