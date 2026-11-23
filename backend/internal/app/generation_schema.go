package app

import (
	_ "embed"
	"encoding/json"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// questionSetSchema is the provider-neutral JSON Schema every LLM adapter
// passes to its provider's structured-output feature.
//
//go:embed question_set.schema.json
var questionSetSchema []byte

// QuestionSetSchema returns a fresh copy of the output schema as a generic map.
func QuestionSetSchema() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(questionSetSchema, &m); err != nil {
		panic("invalid embedded schema: " + err.Error())
	}
	return m
}

// QuestionSetSchemaJSON returns the raw schema bytes.
func QuestionSetSchemaJSON() []byte { return append([]byte(nil), questionSetSchema...) }

// GeneratedQuestion is the wire shape described by QuestionSetSchema.
// Adapters decode LLM output into GeneratedQuestionSet, then call ToDrafts.
type GeneratedQuestion struct {
	Type           string   `json:"type"`
	Prompt         string   `json:"prompt"`
	Options        []string `json:"options"`
	CorrectIndices []int    `json:"correctIndices"`
	Explanation    string   `json:"explanation"`
	SourceHint     string   `json:"sourceHint,omitempty"`
}

// GeneratedQuestionSet is the top-level wire shape.
type GeneratedQuestionSet struct {
	Questions []GeneratedQuestion `json:"questions"`
}

// ToDrafts converts wire questions to domain drafts. Validation happens in
// domain.NewQuestionSet, never here.
func (s GeneratedQuestionSet) ToDrafts() []domain.QuestionDraft {
	out := make([]domain.QuestionDraft, 0, len(s.Questions))
	for _, q := range s.Questions {
		t, err := domain.ParseQuestionType(q.Type)
		if err != nil {
			t = domain.QuestionType(q.Type) // rejected later by the domain
		}
		out = append(out, domain.QuestionDraft{
			Type:        t,
			Prompt:      q.Prompt,
			Options:     q.Options,
			Correct:     q.CorrectIndices,
			Explanation: q.Explanation,
			SourceHint:  q.SourceHint,
		})
	}
	return out
}
