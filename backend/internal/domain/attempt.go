package domain

import (
	"fmt"
	"slices"
	"time"
)

// Answer is the graded response to one drawn question.
type Answer struct {
	QuestionID ID
	Position   int
	Selected   []int
	Score      Score
}

// ChapterAttempt is one finished practice run ("quiz" in the UI) over a chapter's
// QuestionSet (aggregate root). It only exists once submitted (ADR 0011).
// QuestionIDs snapshot the draw order.
//
// The server never sees a quiz start, so StartedAt is the submission time too.
type ChapterAttempt struct {
	ID             ID
	OwnerID        ID
	ChapterID      ID
	RequestedCount int
	QuestionIDs    []ID
	Answers        []Answer
	TotalScore     float64
	MaxScore       int
	StartedAt      time.Time
	SubmittedAt    time.Time
}

// MaxAttemptSize caps how many questions one attempt asks, whatever the pool size.
const MaxAttemptSize = 50

// ValidateAttemptCount checks n is a multiple of 5, at least 5, and at most
// min(poolSize, MaxAttemptSize), where poolSize is the number of questions the
// attempt's scope can draw from.
func ValidateAttemptCount(n, poolSize int) error {
	limit := min(poolSize, MaxAttemptSize)
	if n < MinSetSize || n%SizeStep != 0 || n > limit {
		return Invalid(fmt.Sprintf("question count must be a multiple of %d between %d and %d", SizeStep, MinSetSize, limit))
	}
	return nil
}

// NewChapterAttempt grades a submitted quiz over questionIDs (the questions drawn
// from set, in draw order) and returns the attempt. responses maps question ID to the
// selected option indices; unanswered questions score 0. Scores are always computed
// here, never accepted from the client. The drawn questions must be distinct members
// of set and their count must pass ValidateAttemptCount.
func NewChapterAttempt(id, ownerID ID, set QuestionSet, questionIDs []ID, responses map[ID][]int, now time.Time) (ChapterAttempt, error) {
	if err := ValidateAttemptCount(len(questionIDs), set.Size()); err != nil {
		return ChapterAttempt{}, err
	}
	var problems []string
	drawn := make(map[ID]bool, len(questionIDs))
	for _, qid := range questionIDs {
		if drawn[qid] {
			problems = append(problems, fmt.Sprintf("question %s drawn twice", qid))
		}
		drawn[qid] = true
	}
	for qid := range responses {
		if !drawn[qid] {
			problems = append(problems, fmt.Sprintf("question %s is not part of this quiz", qid))
		}
	}

	answers := make([]Answer, 0, len(questionIDs))
	var total float64
	for pos, qid := range questionIDs {
		q, ok := set.Question(qid)
		if !ok {
			problems = append(problems, fmt.Sprintf("question %s is not in this chapter", qid))
			continue
		}
		selected := slices.Clone(responses[qid])
		slices.Sort(selected)
		selected = slices.Compact(selected)
		if !q.ValidSelection(selected) {
			problems = append(problems, fmt.Sprintf("question %s: selection out of range", qid))
			continue
		}
		if selected == nil {
			selected = []int{}
		}
		s := q.Grade(selected)
		total += s.Points
		answers = append(answers, Answer{QuestionID: qid, Position: pos, Selected: selected, Score: s})
	}
	if err := newValidationError(problems); err != nil {
		return ChapterAttempt{}, err
	}
	return ChapterAttempt{
		ID:             id,
		OwnerID:        ownerID,
		ChapterID:      set.ChapterID,
		RequestedCount: len(questionIDs),
		QuestionIDs:    slices.Clone(questionIDs),
		Answers:        answers,
		TotalScore:     RoundPoints(total),
		MaxScore:       len(questionIDs),
		StartedAt:      now,
		SubmittedAt:    now,
	}, nil
}
