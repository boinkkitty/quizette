package domain

import (
	"fmt"
	"slices"
	"strings"
)

// QuestionType is the answer format of a question.
type QuestionType string

// Question types.
const (
	MCQ       QuestionType = "mcq"
	TrueFalse QuestionType = "true_false"
	MRQ       QuestionType = "mrq"
)

// AllQuestionTypes lists every supported type.
var AllQuestionTypes = []QuestionType{MCQ, TrueFalse, MRQ}

// Option-count rules per type.
const (
	MCQOptions    = 4
	MRQMinOptions = 3
	MRQMaxOptions = 5
)

// TrueFalseOptions are the canonical options for true/false questions.
var TrueFalseOptions = []string{"True", "False"}

// ParseQuestionType validates a raw type string.
func ParseQuestionType(s string) (QuestionType, error) {
	t := QuestionType(strings.ToLower(strings.TrimSpace(s)))
	if !slices.Contains(AllQuestionTypes, t) {
		return "", Invalid(fmt.Sprintf("unknown question type %q", s))
	}
	return t, nil
}

// QuestionDraft is unvalidated question data, e.g. from an LLM.
type QuestionDraft struct {
	Type        QuestionType
	Prompt      string
	Options     []string
	Correct     []int
	Explanation string
	SourceHint  string
}

// Question is an entity inside a QuestionSet. Immutable once created.
type Question struct {
	ID          ID
	Position    int
	Type        QuestionType
	Prompt      string
	Options     []string
	Correct     []int // sorted, unique indices into Options
	Explanation string
	SourceHint  string
}

// newQuestion validates a draft against the per-type invariants.
func newQuestion(id ID, position int, d QuestionDraft) (Question, error) {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	prompt := strings.TrimSpace(d.Prompt)
	if prompt == "" {
		add("prompt is required")
	}
	explanation := strings.TrimSpace(d.Explanation)
	if explanation == "" {
		add("explanation is required")
	}

	options := make([]string, len(d.Options))
	seen := make(map[string]bool, len(d.Options))
	for i, o := range d.Options {
		o = strings.TrimSpace(o)
		options[i] = o
		key := strings.ToLower(o)
		switch {
		case o == "":
			add("option %d is empty", i)
		case seen[key]:
			add("option %q is duplicated", o)
		}
		seen[key] = true
	}

	correct := slices.Clone(d.Correct)
	slices.Sort(correct)
	if len(slices.Compact(slices.Clone(correct))) != len(correct) {
		add("correct answers contain duplicates")
	}
	correct = slices.Compact(correct)
	for _, c := range correct {
		if c < 0 || c >= len(options) {
			add("correct index %d is out of range", c)
		}
	}

	switch d.Type {
	case MCQ:
		if len(options) != MCQOptions {
			add("mcq must have exactly %d options, got %d", MCQOptions, len(options))
		}
		if len(correct) != 1 {
			add("mcq must have exactly 1 correct answer, got %d", len(correct))
		}
	case TrueFalse:
		if len(options) != 2 || !strings.EqualFold(options[0], "true") || !strings.EqualFold(options[1], "false") {
			add(`true_false options must be ["True", "False"]`)
		} else {
			options = slices.Clone(TrueFalseOptions)
		}
		if len(correct) != 1 {
			add("true_false must have exactly 1 correct answer, got %d", len(correct))
		}
	case MRQ:
		if len(options) < MRQMinOptions || len(options) > MRQMaxOptions {
			add("mrq must have %d-%d options, got %d", MRQMinOptions, MRQMaxOptions, len(options))
		}
		// Any number of options can be correct, including all of them. Grading subtracts
		// wrong picks, so ticking everything is penalised whenever some option is wrong.
		if len(correct) < 1 {
			add("mrq must have at least 1 correct answer, got %d", len(correct))
		}
	default:
		add("unknown question type %q", d.Type)
	}

	if err := newValidationError(problems); err != nil {
		return Question{}, err
	}
	return Question{
		ID:          id,
		Position:    position,
		Type:        d.Type,
		Prompt:      prompt,
		Options:     options,
		Correct:     correct,
		Explanation: explanation,
		SourceHint:  strings.TrimSpace(d.SourceHint),
	}, nil
}

// ValidSelection reports whether every selected index refers to an option.
func (q Question) ValidSelection(selected []int) bool {
	for _, s := range selected {
		if s < 0 || s >= len(q.Options) {
			return false
		}
	}
	return true
}
