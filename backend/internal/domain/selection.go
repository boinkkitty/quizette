package domain

import "math/rand/v2"

// QuestionStat summarises a user's history with one question.
type QuestionStat struct {
	Attempts   int
	LastPoints float64
}

// SelectQuestions draws n questions from the set (domain service).
// Priority: never attempted, then previously wrong (last points < 1), then
// the rest; ties are random. The final order is shuffled.
func SelectQuestions(set QuestionSet, history map[ID]QuestionStat, n int, rng *rand.Rand) ([]ID, error) {
	if err := ValidateAttemptCount(n, set.Size()); err != nil {
		return nil, err
	}
	var fresh, wrong, rest []ID
	for _, q := range set.Questions {
		st, seen := history[q.ID]
		switch {
		case !seen || st.Attempts == 0:
			fresh = append(fresh, q.ID)
		case st.LastPoints < 1:
			wrong = append(wrong, q.ID)
		default:
			rest = append(rest, q.ID)
		}
	}
	picked := make([]ID, 0, n)
	for _, tier := range [][]ID{fresh, wrong, rest} {
		rng.Shuffle(len(tier), func(i, j int) { tier[i], tier[j] = tier[j], tier[i] })
		for _, id := range tier {
			if len(picked) == n {
				break
			}
			picked = append(picked, id)
		}
	}
	rng.Shuffle(len(picked), func(i, j int) { picked[i], picked[j] = picked[j], picked[i] })
	return picked, nil
}
