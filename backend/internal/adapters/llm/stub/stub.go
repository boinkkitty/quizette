// Package stub is a deterministic app.QuestionGenerator for tests, E2E runs
// and local development without an API key.
package stub

import (
	"context"
	"fmt"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Generator produces spec.Count valid questions cycling MCQ, True/False, MRQ.
type Generator struct {
	// Delay simulates LLM latency so the UI's processing state is visible.
	Delay time.Duration
}

// Generate implements app.QuestionGenerator.
func (g Generator) Generate(ctx context.Context, in app.GenerateInput) (app.GenerationResult, error) {
	if g.Delay > 0 {
		select {
		case <-time.After(g.Delay):
		case <-ctx.Done():
			return app.GenerationResult{}, fmt.Errorf("stub generate: %w", ctx.Err())
		}
	}
	name := in.Document.Filename()
	drafts := make([]domain.QuestionDraft, in.Spec.Count)
	for i := range drafts {
		n := i + 1
		switch i % 3 {
		case 0:
			drafts[i] = domain.QuestionDraft{
				Type: domain.MCQ, Prompt: fmt.Sprintf("Sample question %d from %s: which option is B?", n, name),
				Options: []string{"Option A", "Option B", "Option C", "Option D"}, Correct: []int{1},
				Explanation: "Option B is the second option.", SourceHint: fmt.Sprintf("Page %d", n),
			}
		case 1:
			drafts[i] = domain.QuestionDraft{
				Type: domain.TrueFalse, Prompt: fmt.Sprintf("Sample statement %d from %s is true.", n, name),
				Options: domain.TrueFalseOptions, Correct: []int{0},
				Explanation: "Stub statements are always true.", SourceHint: fmt.Sprintf("Page %d", n),
			}
		default:
			drafts[i] = domain.QuestionDraft{
				Type: domain.MRQ, Prompt: fmt.Sprintf("Sample question %d from %s: select the vowels.", n, name),
				Options: []string{"A", "B", "E", "F"}, Correct: []int{0, 2},
				Explanation: "A and E are vowels.", SourceHint: fmt.Sprintf("Page %d", n),
			}
		}
	}
	return app.GenerationResult{
		Drafts: drafts,
		Metrics: domain.GenerationMetrics{
			Strategy: "stub", Model: "stub",
			InputTokens: 1000, OutputTokens: 100 * in.Spec.Count, TotalTokens: 1000 + 100*in.Spec.Count,
			LLMMS: g.Delay.Milliseconds(), TotalMS: g.Delay.Milliseconds(),
		},
	}, nil
}
