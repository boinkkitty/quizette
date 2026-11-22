package domain

import "testing"

func TestGrade(t *testing.T) {
	t.Parallel()
	mcq := Question{Type: MCQ, Options: []string{"a", "b", "c", "d"}, Correct: []int{2}}
	tf := Question{Type: TrueFalse, Options: TrueFalseOptions, Correct: []int{0}}
	mrq4 := Question{Type: MRQ, Options: []string{"a", "b", "c", "d"}, Correct: []int{0, 2}}
	mrq5 := Question{Type: MRQ, Options: []string{"a", "b", "c", "d", "e"}, Correct: []int{0, 1, 3}}

	tests := []struct {
		name     string
		q        Question
		selected []int
		want     Score
	}{
		{"mcq correct", mcq, []int{2}, Score{1, true}},
		{"mcq wrong", mcq, []int{1}, Score{0, false}},
		{"mcq none", mcq, nil, Score{0, false}},
		{"mcq correct plus wrong", mcq, []int{2, 1}, Score{0, false}},
		{"mcq duplicate correct", mcq, []int{2, 2}, Score{1, true}},
		{"tf correct", tf, []int{0}, Score{1, true}},
		{"tf wrong", tf, []int{1}, Score{0, false}},
		{"mrq exact", mrq4, []int{0, 2}, Score{1, true}},
		{"mrq exact unordered", mrq4, []int{2, 0}, Score{1, true}},
		{"mrq half", mrq4, []int{0}, Score{0.5, false}},
		{"mrq one right one wrong", mrq4, []int{0, 1}, Score{0, false}},
		{"mrq select all", mrq4, []int{0, 1, 2, 3}, Score{0, false}},
		{"mrq none", mrq4, nil, Score{0, false}},
		{"mrq only wrong floors at zero", mrq4, []int{1, 3}, Score{0, false}},
		{"mrq 3 correct, 2 right", mrq5, []int{0, 1}, Score{0.6667, false}},
		{"mrq 3 correct, all right + 1 wrong", mrq5, []int{0, 1, 3, 4}, Score{0.6667, false}},
		{"mrq out of range ignored", mrq4, []int{0, 2, 9, -1}, Score{1, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.q.Grade(tt.selected); got != tt.want {
				t.Errorf("Grade(%v) = %+v, want %+v", tt.selected, got, tt.want)
			}
		})
	}
}
