package domain

import (
	"math/rand/v2"
	"slices"
)

// SplitAcrossChapters splits n questions across chapters in proportion to their
// sizes (question counts) with largest-remainder rounding, and returns how many to
// draw from each, in the order of sizes. A remainder tie goes to the earlier chapter,
// so callers must pass chapters in a stable order. It assumes 0 <= n <= the sum of
// sizes; no chapter is then given more than its size.
//
// The frontend previews this split, so a change here must be mirrored there.
func SplitAcrossChapters(n int, sizes []int) []int {
	total := 0
	for _, s := range sizes {
		total += s
	}
	out := make([]int, len(sizes))
	if total == 0 {
		return out
	}
	// Chapter i's exact share is n*sizes[i]/total. Keep it as integer quotient and
	// remainder so the split never depends on floating-point rounding.
	rem := make([]int, len(sizes))
	given := 0
	for i, s := range sizes {
		out[i], rem[i] = n*s/total, n*s%total
		given += out[i]
	}
	order := make([]int, len(sizes))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return rem[b] - rem[a] })
	for _, i := range order[:n-given] {
		out[i]++
	}
	return out
}

// DrawQuestions draws n distinct questions from the pool at random (domain service),
// after checking n against the count rule. n is split across the pool's sets with
// SplitAcrossChapters, each set's share is drawn at random, and the whole draw is
// then shuffled so chapters are mixed. The result is in draw order.
func DrawQuestions(pool Pool, n int, rng *rand.Rand) ([]ID, error) {
	if err := ValidateAttemptCount(n, pool.Size()); err != nil {
		return nil, err
	}
	sizes := make([]int, len(pool))
	for i, s := range pool {
		sizes[i] = s.Size()
	}
	out := make([]ID, 0, n)
	for i, k := range SplitAcrossChapters(n, sizes) {
		ids := make([]ID, len(pool[i].Questions))
		for j, q := range pool[i].Questions {
			ids[j] = q.ID
		}
		rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
		out = append(out, ids[:k]...)
	}
	rng.Shuffle(len(out), func(a, b int) { out[a], out[b] = out[b], out[a] })
	return out, nil
}
