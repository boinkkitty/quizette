// Package system provides production implementations of small infrastructure
// ports: clock, ID generation and password hashing.
package system

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Clock is the wall clock (UTC).
type Clock struct{}

// Now implements app.Clock.
func (Clock) Now() time.Time { return time.Now().UTC() }

// UUIDv7 generates time-ordered UUIDs.
type UUIDv7 struct{}

// NewID implements app.IDGenerator.
func (UUIDv7) NewID() domain.ID { return domain.ID(uuid.Must(uuid.NewV7()).String()) }

// Bcrypt hashes passwords with bcrypt.
type Bcrypt struct{ Cost int }

// Hash implements app.PasswordHasher.
func (b Bcrypt) Hash(p string) (string, error) {
	cost := b.Cost
	if cost == 0 {
		cost = 12
	}
	h, err := bcrypt.GenerateFromPassword([]byte(p), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(h), nil
}

// Compare implements app.PasswordHasher.
func (Bcrypt) Compare(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}
