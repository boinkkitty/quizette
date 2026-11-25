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

// ScopeType says what an attempt draws from.
type ScopeType string

// Scope types.
const (
	ScopeChapter ScopeType = "chapter"
	ScopeTopic   ScopeType = "topic"
)

// AttemptScope is what an attempt draws from: one chapter, or a whole topic (its
// ready chapters). ID is the chapter's or the topic's ID, according to Type.
type AttemptScope struct {
	Type ScopeType
	ID   ID
}

// ChapterScope returns the scope of a chapter quiz.
func ChapterScope(chapterID ID) AttemptScope { return AttemptScope{Type: ScopeChapter, ID: chapterID} }

// TopicScope returns the scope of a topic quiz.
func TopicScope(topicID ID) AttemptScope { return AttemptScope{Type: ScopeTopic, ID: topicID} }

// Pool is the question sets an attempt scope draws from: one set for a chapter
// scope, one per ready chapter for a topic scope. Its order is the order the draw
// splits questions in (see SplitAcrossChapters).
type Pool []QuestionSet

// Size returns the number of questions across the pool.
func (p Pool) Size() int {
	n := 0
	for _, s := range p {
		n += s.Size()
	}
	return n
}

// Question finds a question in the pool by ID.
func (p Pool) Question(id ID) (Question, bool) {
	for _, s := range p {
		if q, ok := s.Question(id); ok {
			return q, true
		}
	}
	return Question{}, false
}

// ChapterOf returns the chapter the question id comes from, if it is in the pool.
func (p Pool) ChapterOf(id ID) (ID, bool) {
	for _, s := range p {
		if _, ok := s.Question(id); ok {
			return s.ChapterID, true
		}
	}
	return "", false
}

// Attempt is one finished practice run ("quiz" in the UI) over an attempt scope
// (aggregate root). It only exists once submitted (ADR 0011). QuestionIDs snapshot
// the draw order, so chapters added to a topic later don't change it (ADR 0010).
//
// The server never sees a quiz start, so StartedAt is the submission time too.
type Attempt struct {
	ID             ID
	OwnerID        ID
	Scope          AttemptScope
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

// NewAttempt grades a submitted quiz over questionIDs (the questions drawn from
// pool, the scope's question sets, in draw order) and returns the attempt. responses
// maps question ID to the selected option indices; unanswered questions score 0.
// Scores are always computed here, never accepted from the client. The drawn
// questions must be distinct members of pool and their count must pass
// ValidateAttemptCount against the pool's size.
func NewAttempt(id, ownerID ID, scope AttemptScope, pool Pool, questionIDs []ID, responses map[ID][]int, now time.Time) (Attempt, error) {
	if err := ValidateAttemptCount(len(questionIDs), pool.Size()); err != nil {
		return Attempt{}, err
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
		q, ok := pool.Question(qid)
		if !ok {
			problems = append(problems, fmt.Sprintf("question %s is not in this %s", qid, scope.Type))
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
		return Attempt{}, err
	}
	return Attempt{
		ID:             id,
		OwnerID:        ownerID,
		Scope:          scope,
		RequestedCount: len(questionIDs),
		QuestionIDs:    slices.Clone(questionIDs),
		Answers:        answers,
		TotalScore:     RoundPoints(total),
		MaxScore:       len(questionIDs),
		StartedAt:      now,
		SubmittedAt:    now,
	}, nil
}
