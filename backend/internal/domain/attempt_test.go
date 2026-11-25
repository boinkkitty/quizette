package domain

import (
	"errors"
	"testing"
	"time"
)

func readySet(t *testing.T, n int) QuestionSet {
	t.Helper()
	return chapterSet(t, "c", n, seqID())
}

// chapterSet builds chapterID's question set of n questions, with IDs from newID.
func chapterSet(t *testing.T, chapterID ID, n int, newID func() ID) QuestionSet {
	t.Helper()
	spec, err := NewQuestionSpec(n, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, _, err := NewQuestionSet("s-"+chapterID, chapterID, spec, drafts(n), newID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func ids(set QuestionSet) []ID {
	out := make([]ID, len(set.Questions))
	for i, q := range set.Questions {
		out[i] = q.ID
	}
	return out
}

func TestNewAttemptGradesChapterQuiz(t *testing.T) {
	t.Parallel()
	set := readySet(t, 5) // q1 mcq(b), q2 tf(true), q3 mrq(a,c), q4 mcq, q5 tf
	now := time.Now()

	responses := map[ID][]int{
		"q1": {1},    // correct
		"q2": {1},    // wrong
		"q3": {0},    // half
		"q4": {1, 1}, // correct (dup)
		// q5 unanswered
	}
	a, err := NewAttempt("a", "u", ChapterScope("c"), Pool{set}, ids(set), responses, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Scope != ChapterScope("c") || !a.SubmittedAt.Equal(now) {
		t.Fatalf("attempt = %+v", a)
	}
	if a.TotalScore != 2.5 || a.MaxScore != 5 {
		t.Errorf("score %v/%d, want 2.5/5", a.TotalScore, a.MaxScore)
	}
	if got := a.Answers[4]; got.Score.Points != 0 || len(got.Selected) != 0 {
		t.Errorf("unanswered = %+v", got)
	}
}

func TestNewAttemptRejectsBadDraws(t *testing.T) {
	t.Parallel()
	set := readySet(t, 10)
	five := ids(set)[:5]
	tests := map[string]struct {
		questionIDs []ID
		responses   map[ID][]int
	}{
		"answer to an undrawn question": {five, map[ID][]int{"q9": {0}}},
		"selection out of range":        {five, map[ID][]int{"q1": {7}}},
		"question from another chapter": {append(ids(set)[:4:4], "zzz"), nil},
		"question drawn twice":          {append(ids(set)[:4:4], "q1"), nil},
		"count not a multiple of 5":     {ids(set)[:4], nil},
	}
	for name, tt := range tests {
		var verr *ValidationError
		if _, err := NewAttempt("a", "u", ChapterScope("c"), Pool{set}, tt.questionIDs, tt.responses, time.Now()); !errors.As(err, &verr) {
			t.Errorf("%s: got %v, want a ValidationError", name, err)
		}
	}
}

func TestNewAttemptOverTopicPool(t *testing.T) {
	t.Parallel()
	newID := seqID()
	cells, genes := chapterSet(t, "cells", 5, newID), chapterSet(t, "genes", 5, newID) // q1..q5, q6..q10
	pool := Pool{cells, genes}
	scope := TopicScope("bio")

	drawn := []ID{"q1", "q6", "q2", "q7", "q3", "q8", "q4", "q9", "q5", "q10"}
	responses := map[ID][]int{"q1": {1}, "q6": {1}} // both MCQs, both correct
	a, err := NewAttempt("a", "u", scope, pool, drawn, responses, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Scope != scope || a.TotalScore != 2 || a.MaxScore != 10 || len(a.Answers) != 10 || a.Answers[1].QuestionID != "q6" {
		t.Fatalf("attempt = %+v", a)
	}
	if ch, ok := pool.ChapterOf("q7"); !ok || ch != "genes" {
		t.Errorf("ChapterOf(q7) = %q, %v; want genes", ch, ok)
	}

	other := chapterSet(t, "elsewhere", 5, newID) // q11..q15
	if _, err := NewAttempt("a", "u", scope, pool, append(drawn[:9:9], other.Questions[0].ID), nil, time.Now()); !isValidation(err) {
		t.Errorf("question outside the topic: got %v, want a ValidationError", err)
	}
}

func isValidation(err error) bool {
	var verr *ValidationError
	return errors.As(err, &verr)
}

func TestValidateAttemptCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n, size int
		ok      bool
	}{{5, 5, true}, {10, 20, true}, {20, 20, true}, {25, 20, false}, {0, 20, false}, {7, 20, false}, {50, 60, true}, {55, 60, false}}
	for _, tt := range tests {
		if err := ValidateAttemptCount(tt.n, tt.size); (err == nil) != tt.ok {
			t.Errorf("ValidateAttemptCount(%d,%d) = %v", tt.n, tt.size, err)
		}
	}
}

func TestNormalizeEmailAndPassword(t *testing.T) {
	t.Parallel()
	if e, err := NormalizeEmail("  Foo@Example.COM "); err != nil || e != "foo@example.com" {
		t.Errorf("got %q %v", e, err)
	}
	for _, bad := range []string{"", "nope", "a@", "Name <a@b.c>"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	if ValidatePassword("short") == nil || ValidatePassword("longenough") != nil {
		t.Error("password rules")
	}
}
