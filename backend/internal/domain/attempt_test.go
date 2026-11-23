package domain

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func readySet(t *testing.T, n int) QuestionSet {
	t.Helper()
	spec, err := NewQuestionSpec(n, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, _, err := NewQuestionSet("s", "c", spec, drafts(n), seqID(), time.Now())
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

func TestChapterAttemptSubmit(t *testing.T) {
	t.Parallel()
	set := readySet(t, 5) // q1 mcq(b), q2 tf(true), q3 mrq(a,c), q4 mcq, q5 tf
	now := time.Now()

	a := NewChapterAttempt("a", "u", "c", ids(set), now)
	responses := map[ID][]int{
		"q1": {1},    // correct
		"q2": {1},    // wrong
		"q3": {0},    // half
		"q4": {1, 1}, // correct (dup)
		// q5 unanswered
	}
	if err := a.Submit(set, responses, now); err != nil {
		t.Fatal(err)
	}
	if a.Status != AttemptSubmitted || a.SubmittedAt == nil {
		t.Fatalf("status %s", a.Status)
	}
	if a.TotalScore != 2.5 || a.MaxScore != 5 {
		t.Errorf("score %v/%d, want 2.5/5", a.TotalScore, a.MaxScore)
	}
	if got := a.Answers[4]; got.Score.Points != 0 || len(got.Selected) != 0 {
		t.Errorf("unanswered = %+v", got)
	}
	if err := a.Submit(set, responses, now); !errors.Is(err, ErrConflict) {
		t.Errorf("double submit: %v", err)
	}
}

func TestChapterAttemptSubmitRejectsForeignAndOutOfRange(t *testing.T) {
	t.Parallel()
	set := readySet(t, 5)
	a := NewChapterAttempt("a", "u", "c", ids(set), time.Now())
	if err := a.Submit(set, map[ID][]int{"zzz": {0}}, time.Now()); err == nil {
		t.Error("foreign question: want error")
	}
	if err := a.Submit(set, map[ID][]int{"q1": {7}}, time.Now()); err == nil {
		t.Error("out of range: want error")
	}
	if a.Status != AttemptInProgress {
		t.Error("failed submit must not change status")
	}
}

func TestValidateAttemptCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n, size int
		ok      bool
	}{{5, 5, true}, {10, 20, true}, {20, 20, true}, {25, 20, false}, {0, 20, false}, {7, 20, false}}
	for _, tt := range tests {
		if err := ValidateAttemptCount(tt.n, tt.size); (err == nil) != tt.ok {
			t.Errorf("ValidateAttemptCount(%d,%d) = %v", tt.n, tt.size, err)
		}
	}
}

func TestSelectQuestions(t *testing.T) {
	t.Parallel()
	set := readySet(t, 15)
	rng := rand.New(rand.NewPCG(1, 2))

	history := map[ID]QuestionStat{}
	// q1..q10 attempted correctly, q11..q13 wrong last time, q14,q15 fresh.
	for i, q := range set.Questions {
		switch {
		case i < 10:
			history[q.ID] = QuestionStat{Attempts: 1, LastPoints: 1}
		case i < 13:
			history[q.ID] = QuestionStat{Attempts: 2, LastPoints: 0.5}
		}
	}
	got, err := SelectQuestions(set, history, 5, rng)
	if err != nil {
		t.Fatal(err)
	}
	want := []ID{"q11", "q12", "q13", "q14", "q15"}
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	if !slices.Equal(sorted, want) {
		t.Errorf("got %v, want fresh+wrong %v", sorted, want)
	}

	all, err := SelectQuestions(set, nil, 15, rng)
	if err != nil || len(all) != 15 {
		t.Fatalf("all: %v %v", all, err)
	}
	slices.Sort(all)
	if len(slices.Compact(all)) != 15 {
		t.Error("duplicates drawn")
	}
	if _, err := SelectQuestions(set, nil, 20, rng); err == nil {
		t.Error("n > size: want error")
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
