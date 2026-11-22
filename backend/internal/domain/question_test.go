package domain

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

func validDraft(t QuestionType) QuestionDraft {
	switch t {
	case MCQ:
		return QuestionDraft{Type: MCQ, Prompt: "Pick b", Options: []string{"a", "b", "c", "d"}, Correct: []int{1}, Explanation: "because"}
	case TrueFalse:
		return QuestionDraft{Type: TrueFalse, Prompt: "Sky is blue", Options: []string{"true", "FALSE"}, Correct: []int{0}, Explanation: "because"}
	default:
		return QuestionDraft{Type: MRQ, Prompt: "Pick a and c", Options: []string{"a", "b", "c", "d"}, Correct: []int{2, 0}, Explanation: "because"}
	}
}

func TestNewQuestion(t *testing.T) {
	t.Parallel()
	mod := func(d QuestionDraft, f func(*QuestionDraft)) QuestionDraft { f(&d); return d }
	tests := []struct {
		name    string
		draft   QuestionDraft
		wantErr bool
	}{
		{"valid mcq", validDraft(MCQ), false},
		{"valid tf normalises case", validDraft(TrueFalse), false},
		{"valid mrq", validDraft(MRQ), false},
		{"mrq 3 options 2 correct", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Options = []string{"a", "b", "c"}; d.Correct = []int{0, 1} }), false},
		{"mrq 5 options 4 correct", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Options = []string{"a", "b", "c", "d", "e"}; d.Correct = []int{0, 1, 2, 3} }), false},
		{"mrq 2 options", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Options = []string{"a", "b"}; d.Correct = []int{0, 1} }), true},
		{"mrq 6 options", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Options = []string{"a", "b", "c", "d", "e", "f"} }), true},
		{"mrq single correct", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Correct = []int{0} }), true},
		{"mrq all correct", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Correct = []int{0, 1, 2, 3} }), true},
		{"mcq 3 options", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Options = []string{"a", "b", "c"} }), true},
		{"mcq two correct", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Correct = []int{0, 1} }), true},
		{"mcq index out of range", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Correct = []int{4} }), true},
		{"tf wrong options", mod(validDraft(TrueFalse), func(d *QuestionDraft) { d.Options = []string{"Yes", "No"} }), true},
		{"empty prompt", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Prompt = "  " }), true},
		{"empty explanation", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Explanation = "" }), true},
		{"duplicate options", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Options = []string{"a", "A", "c", "d"} }), true},
		{"empty option", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Options = []string{"a", "", "c", "d"} }), true},
		{"duplicate correct", mod(validDraft(MRQ), func(d *QuestionDraft) { d.Correct = []int{0, 0, 2} }), true},
		{"unknown type", mod(validDraft(MCQ), func(d *QuestionDraft) { d.Type = "essay" }), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q, err := newQuestion("q1", 0, tt.draft)
			if tt.wantErr {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.IsSorted(q.Correct) {
				t.Errorf("correct not sorted: %v", q.Correct)
			}
			if q.Type == TrueFalse && !slices.Equal(q.Options, TrueFalseOptions) {
				t.Errorf("tf options not canonical: %v", q.Options)
			}
		})
	}
}

func seqID() func() ID {
	n := 0
	return func() ID { n++; return ID(fmt.Sprintf("q%d", n)) }
}

func drafts(n int) []QuestionDraft {
	out := make([]QuestionDraft, n)
	for i := range out {
		d := validDraft(AllQuestionTypes[i%3])
		d.Prompt = fmt.Sprintf("%s %d", d.Prompt, i)
		out[i] = d
	}
	return out
}

func TestNewQuestionSpec(t *testing.T) {
	t.Parallel()
	for _, n := range []int{5, 10, 15, 20, 25, 30} {
		if _, err := NewQuestionSpec(n, nil); err != nil {
			t.Errorf("count %d: %v", n, err)
		}
	}
	for _, n := range []int{0, 4, 6, 35, -5} {
		if _, err := NewQuestionSpec(n, nil); err == nil {
			t.Errorf("count %d: want error", n)
		}
	}
	if _, err := NewQuestionSpec(5, []QuestionType{"essay"}); err == nil {
		t.Error("unknown type: want error")
	}
}

func TestNewQuestionSet(t *testing.T) {
	t.Parallel()
	spec, _ := NewQuestionSpec(5, nil)
	now := time.Now()

	t.Run("exact count", func(t *testing.T) {
		set, discarded, err := NewQuestionSet("s", "c", spec, drafts(5), seqID(), now)
		if err != nil || len(discarded) != 0 || set.Size() != 5 {
			t.Fatalf("got size=%d discarded=%v err=%v", set.Size(), discarded, err)
		}
		for i, q := range set.Questions {
			if q.Position != i {
				t.Errorf("question %d has position %d", i, q.Position)
			}
		}
	})
	t.Run("extra drafts truncated", func(t *testing.T) {
		set, _, err := NewQuestionSet("s", "c", spec, drafts(8), seqID(), now)
		if err != nil || set.Size() != 5 {
			t.Fatalf("size=%d err=%v", set.Size(), err)
		}
	})
	t.Run("invalid drafts discarded but enough remain", func(t *testing.T) {
		ds := drafts(7)
		ds[1].Correct = []int{99}
		ds[3].Prompt = ""
		set, discarded, err := NewQuestionSet("s", "c", spec, ds, seqID(), now)
		if err != nil || set.Size() != 5 || len(discarded) != 2 {
			t.Fatalf("size=%d discarded=%v err=%v", set.Size(), discarded, err)
		}
	})
	t.Run("too few valid", func(t *testing.T) {
		ds := drafts(5)
		ds[0].Options = nil
		_, _, err := NewQuestionSet("s", "c", spec, ds, seqID(), now)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want ValidationError, got %v", err)
		}
	})
	t.Run("duplicate prompts discarded", func(t *testing.T) {
		ds := drafts(5)
		ds[4].Prompt = ds[0].Prompt
		ds[4].Type, ds[4].Options, ds[4].Correct = ds[0].Type, ds[0].Options, ds[0].Correct
		if _, _, err := NewQuestionSet("s", "c", spec, ds, seqID(), now); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("type not in mix", func(t *testing.T) {
		mcqOnly, _ := NewQuestionSpec(5, []QuestionType{MCQ})
		if _, _, err := NewQuestionSet("s", "c", mcqOnly, drafts(5), seqID(), now); err == nil {
			t.Fatal("want error")
		}
	})
}
