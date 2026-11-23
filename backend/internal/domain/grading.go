package domain

import (
	"math"
	"slices"
)

// Score is the result of grading one answer. Points is in [0, 1].
type Score struct {
	Points  float64
	Correct bool
}

// Grade scores a selection against the question.
//
//   - MCQ / True-False: 1 point for selecting exactly the correct option, else 0.
//   - MRQ: max(0, (correctSelected - wrongSelected) / totalCorrect).
//     Each right pick earns credit, each wrong pick cancels one, floor at 0,
//     so selecting everything never beats selecting the right subset.
//
// Duplicate and out-of-range indices are ignored.
func (q Question) Grade(selected []int) Score {
	picked := make(map[int]bool, len(selected))
	for _, s := range selected {
		if s >= 0 && s < len(q.Options) {
			picked[s] = true
		}
	}
	var right, wrong int
	for s := range picked {
		if slices.Contains(q.Correct, s) {
			right++
		} else {
			wrong++
		}
	}

	var points float64
	switch q.Type {
	case MCQ, TrueFalse:
		if right == 1 && wrong == 0 {
			points = 1
		}
	case MRQ:
		if len(q.Correct) > 0 {
			points = math.Max(0, float64(right-wrong)/float64(len(q.Correct)))
		}
	}
	points = RoundPoints(points)
	return Score{Points: points, Correct: points == 1}
}

// RoundPoints rounds to 4 decimal places (matches numeric(…,4) storage).
func RoundPoints(p float64) float64 {
	return math.Round(p*10000) / 10000
}
