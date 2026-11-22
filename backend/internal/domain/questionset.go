package domain

import (
	"fmt"
	"slices"
	"time"
)

// Bank-size rules shared by uploads, prompts, validation and attempts.
const (
	MinSetSize = 5
	MaxSetSize = 30
	SizeStep   = 5
)

// QuestionSpec describes what to generate.
type QuestionSpec struct {
	Count   int
	TypeMix []QuestionType
}

// NewQuestionSpec validates count (5..30, multiple of 5). An empty mix means all types.
func NewQuestionSpec(count int, mix []QuestionType) (QuestionSpec, error) {
	if count < MinSetSize || count > MaxSetSize || count%SizeStep != 0 {
		return QuestionSpec{}, Invalid(fmt.Sprintf("question count must be a multiple of %d between %d and %d", SizeStep, MinSetSize, MaxSetSize))
	}
	if len(mix) == 0 {
		mix = slices.Clone(AllQuestionTypes)
	}
	for _, t := range mix {
		if !slices.Contains(AllQuestionTypes, t) {
			return QuestionSpec{}, Invalid(fmt.Sprintf("unknown question type %q", t))
		}
	}
	return QuestionSpec{Count: count, TypeMix: mix}, nil
}

// QuestionSet is a chapter's immutable question bank (aggregate root).
type QuestionSet struct {
	ID        ID
	ChapterID ID
	Questions []Question
	CreatedAt time.Time
}

// NewQuestionSet is the anti-corruption boundary for generated questions.
// Invalid drafts are discarded (their problems are returned as discarded);
// the set is created from the first spec.Count valid drafts. If fewer than
// spec.Count are valid, a ValidationError listing every problem is returned.
func NewQuestionSet(id, chapterID ID, spec QuestionSpec, drafts []QuestionDraft, newID func() ID, now time.Time) (set QuestionSet, discarded []string, err error) {
	questions := make([]Question, 0, spec.Count)
	for i, d := range drafts {
		if len(questions) == spec.Count {
			break
		}
		if d.Type != "" && !slices.Contains(spec.TypeMix, d.Type) {
			discarded = append(discarded, fmt.Sprintf("question %d: type %q not requested", i+1, d.Type))
			continue
		}
		q, qerr := newQuestion(newID(), len(questions), d)
		if qerr != nil {
			discarded = append(discarded, fmt.Sprintf("question %d: %v", i+1, qerr))
			continue
		}
		if slices.ContainsFunc(questions, func(o Question) bool { return o.Prompt == q.Prompt }) {
			discarded = append(discarded, fmt.Sprintf("question %d: duplicate prompt", i+1))
			continue
		}
		questions = append(questions, q)
	}
	if len(questions) < spec.Count {
		problems := append([]string{fmt.Sprintf("need %d valid questions, got %d", spec.Count, len(questions))}, discarded...)
		return QuestionSet{}, discarded, &ValidationError{Problems: problems}
	}
	return QuestionSet{ID: id, ChapterID: chapterID, Questions: questions, CreatedAt: now}, discarded, nil
}

// Question finds a question by ID.
func (s QuestionSet) Question(id ID) (Question, bool) {
	i := slices.IndexFunc(s.Questions, func(q Question) bool { return q.ID == id })
	if i < 0 {
		return Question{}, false
	}
	return s.Questions[i], true
}

// Size returns the number of questions.
func (s QuestionSet) Size() int { return len(s.Questions) }
