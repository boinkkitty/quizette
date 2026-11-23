package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

const maxNameLen = 120

// Topic groups a user's chapters. Uniqueness of Name per owner is enforced by
// the repository (a set-wide rule), surfaced as ErrConflict.
type Topic struct {
	ID        ID
	OwnerID   ID
	Name      string
	CreatedAt time.Time
}

// NewTopic validates and creates a Topic.
func NewTopic(id, ownerID ID, name string, now time.Time) (Topic, error) {
	name, err := cleanName("topic name", name)
	if err != nil {
		return Topic{}, err
	}
	return Topic{ID: id, OwnerID: ownerID, Name: name, CreatedAt: now}, nil
}

// Rename changes the topic name.
func (t *Topic) Rename(name string) error {
	name, err := cleanName("topic name", name)
	if err != nil {
		return err
	}
	t.Name = name
	return nil
}

func cleanName(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", Invalid(field + " is required")
	case utf8.RuneCountInString(s) > maxNameLen:
		return "", Invalid(field + " must be at most 120 characters")
	}
	return s, nil
}
