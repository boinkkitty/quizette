package domain

import (
	"fmt"
	"time"
)

// ChapterStatus is the user-facing readiness of a chapter.
type ChapterStatus string

// Chapter statuses.
const (
	ChapterPending ChapterStatus = "pending"
	ChapterReady   ChapterStatus = "ready"
	ChapterFailed  ChapterStatus = "failed"
)

// Chapter is created by one document upload and owns (by ID) one QuestionSet.
// DeletedAt is set on a deleted chapter: one kept, hidden, because an attempt refers
// to it (ADR 0010).
type Chapter struct {
	ID        ID
	TopicID   ID
	OwnerID   ID
	Title     string
	Status    ChapterStatus
	CreatedAt time.Time
	DeletedAt *time.Time
}

// NewChapter creates a pending chapter.
func NewChapter(id, topicID, ownerID ID, title string, now time.Time) (Chapter, error) {
	title, err := cleanName("chapter title", title)
	if err != nil {
		return Chapter{}, err
	}
	return Chapter{ID: id, TopicID: topicID, OwnerID: ownerID, Title: title, Status: ChapterPending, CreatedAt: now}, nil
}

// MarkReady records that the chapter's question set exists.
func (c *Chapter) MarkReady() error {
	if c.Status != ChapterPending {
		return fmt.Errorf("%w: chapter %s -> %s", ErrInvalidTransition, c.Status, ChapterReady)
	}
	c.Status = ChapterReady
	return nil
}

// MarkFailed records that generation failed.
func (c *Chapter) MarkFailed() error {
	if c.Status != ChapterPending {
		return fmt.Errorf("%w: chapter %s -> %s", ErrInvalidTransition, c.Status, ChapterFailed)
	}
	c.Status = ChapterFailed
	return nil
}

// Rename changes the chapter title.
func (c *Chapter) Rename(title string) error {
	title, err := cleanName("chapter title", title)
	if err != nil {
		return err
	}
	c.Title = title
	return nil
}
