package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Password length rules (bcrypt ignores bytes beyond 72).
const (
	MinPasswordLen = 8
	MaxPasswordLen = 72
)

// User is an account (Identity subdomain).
type User struct {
	ID           ID
	Email        string
	PasswordHash string // empty for OAuth-only accounts
	Name         string
	AvatarURL    string
	CreatedAt    time.Time
}

// HasPassword reports whether password login is enabled.
func (u User) HasPassword() bool { return u.PasswordHash != "" }

// NormalizeEmail validates and lowercases an email address.
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > 254 {
		return "", Invalid("email is invalid")
	}
	return s, nil
}

// ValidatePassword checks password length.
func ValidatePassword(p string) error {
	if utf8.RuneCountInString(p) < MinPasswordLen || len(p) > MaxPasswordLen {
		return Invalid("password must be 8-72 characters")
	}
	return nil
}

// OAuthIdentity is the verified profile returned by an OAuth provider.
type OAuthIdentity struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	Name           string
	AvatarURL      string
}
