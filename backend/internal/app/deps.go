package app

import (
	"log/slog"
	"math/rand/v2"
)

// Deps are the ports shared by all services.
type Deps struct {
	Repos  Repos // non-transactional repositories for reads and single writes
	UoW    UnitOfWork
	Clock  Clock
	IDs    IDGenerator
	Logger *slog.Logger
	// NewRand returns a random source for question selection (overridable in tests).
	NewRand func() *rand.Rand
}

func (d Deps) rand() *rand.Rand {
	if d.NewRand != nil {
		return d.NewRand()
	}
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // selection order is not security-sensitive
}
