package stub_test

import (
	"context"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/stub"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

func TestStubProducesValidSet(t *testing.T) {
	t.Parallel()
	doc, _ := domain.NewDocument("x.pdf", []byte("%PDF-1.4"))
	for _, n := range []int{5, 30} {
		spec, _ := domain.NewQuestionSpec(n, nil)
		res, err := stub.Generator{}.Generate(context.Background(), app.GenerateInput{Document: doc, Spec: spec})
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		if _, discarded, err := domain.NewQuestionSet("s", "c", spec, res.Drafts, func() domain.ID { i++; return domain.ID(rune(i)) }, time.Now()); err != nil || len(discarded) > 0 {
			t.Fatalf("n=%d: %v %v", n, err, discarded)
		}
	}
}
