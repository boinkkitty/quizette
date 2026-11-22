package domain

import (
	"fmt"
	"slices"
	"time"
)

// AttemptStatus is the ChapterAttempt lifecycle state.
type AttemptStatus string

// Attempt statuses.
const (
	AttemptInProgress AttemptStatus = "in_progress"
	AttemptSubmitted  AttemptStatus = "submitted"
)

// Answer is the graded response to one drawn question.
type Answer struct {
	QuestionID ID
	Position   int
	Selected   []int
	Score      Score
}

// ChapterAttempt is one practice run ("quiz" in the UI) over a chapter's
// QuestionSet (aggregate root). QuestionIDs snapshot the draw order.
type ChapterAttempt struct {
	ID             ID
	OwnerID        ID
	ChapterID      ID
	RequestedCount int
	QuestionIDs    []ID
	Answers        []Answer
	Status         AttemptStatus
	TotalScore     float64
	MaxScore       int
	StartedAt      time.Time
	SubmittedAt    *time.Time
}

// ValidateAttemptCount checks n is a multiple of 5, at least 5, and fits the set.
func ValidateAttemptCount(n, setSize int) error {
	if n < MinSetSize || n%SizeStep != 0 || n > setSize {
		return Invalid(fmt.Sprintf("question count must be a multiple of %d between %d and %d", SizeStep, MinSetSize, setSize))
	}
	return nil
}

// NewChapterAttempt starts an attempt over the drawn questions.
func NewChapterAttempt(id, ownerID, chapterID ID, questionIDs []ID, now time.Time) ChapterAttempt {
	return ChapterAttempt{
		ID:             id,
		OwnerID:        ownerID,
		ChapterID:      chapterID,
		RequestedCount: len(questionIDs),
		QuestionIDs:    slices.Clone(questionIDs),
		Status:         AttemptInProgress,
		MaxScore:       len(questionIDs),
		StartedAt:      now,
	}
}

// Submit grades responses (question ID -> selected option indices) against
// the set. Unanswered questions score 0. Scores are always computed here,
// never accepted from the client.
func (a *ChapterAttempt) Submit(set QuestionSet, responses map[ID][]int, now time.Time) error {
	if a.Status != AttemptInProgress {
		return fmt.Errorf("%w: attempt already submitted", ErrConflict)
	}
	if set.ChapterID != a.ChapterID {
		return Invalid("question set does not belong to this chapter")
	}
	var problems []string
	for qid := range responses {
		if !slices.Contains(a.QuestionIDs, qid) {
			problems = append(problems, fmt.Sprintf("question %s is not part of this attempt", qid))
		}
	}

	answers := make([]Answer, 0, len(a.QuestionIDs))
	var total float64
	for pos, qid := range a.QuestionIDs {
		q, ok := set.Question(qid)
		if !ok {
			return fmt.Errorf("%w: question %s missing from set", ErrNotFound, qid)
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
		return err
	}

	a.Answers = answers
	a.TotalScore = RoundPoints(total)
	a.Status = AttemptSubmitted
	a.SubmittedAt = &now
	return nil
}
