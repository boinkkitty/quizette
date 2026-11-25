package domain

import (
	"math/rand/v2"
	"slices"
)

// DrawQuestions draws n distinct questions from the set at random (domain
// service), after checking n against the count rule. The result is in draw order.
func DrawQuestions(set QuestionSet, n int, rng *rand.Rand) ([]ID, error) {
	if err := ValidateAttemptCount(n, set.Size()); err != nil {
		return nil, err
	}
	pool := make([]ID, len(set.Questions))
	for i, q := range set.Questions {
		pool[i] = q.ID
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	return slices.Clip(pool[:n]), nil
}
