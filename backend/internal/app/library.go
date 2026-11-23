package app

import (
	"context"
	"fmt"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// LibraryService manages topics, chapters and question-set review.
type LibraryService struct{ Deps }

// CreateTopic creates an empty topic.
func (s *LibraryService) CreateTopic(ctx context.Context, ownerID domain.ID, name string) (domain.Topic, error) {
	t, err := domain.NewTopic(s.IDs.NewID(), ownerID, name, s.Clock.Now())
	if err != nil {
		return domain.Topic{}, err
	}
	return t, s.Repos.Topics.Create(ctx, t)
}

// ListTopics lists the user's topics.
func (s *LibraryService) ListTopics(ctx context.Context, ownerID domain.ID) ([]TopicSummary, error) {
	return s.Repos.Topics.List(ctx, ownerID)
}

// ListChapters lists the user's chapters across topics, narrowed by filter. A status
// other than pending, ready or failed returns a *domain.ValidationError.
func (s *LibraryService) ListChapters(ctx context.Context, ownerID domain.ID, filter ChapterFilter) ([]ChapterSummary, error) {
	switch filter.Status {
	case "", domain.ChapterPending, domain.ChapterReady, domain.ChapterFailed:
	default:
		return nil, domain.Invalid(fmt.Sprintf("unknown chapter status %q", filter.Status))
	}
	return s.Repos.Chapters.ListByOwner(ctx, ownerID, filter)
}

// TopicDetail is a topic with its chapters.
type TopicDetail struct {
	Topic    domain.Topic
	Chapters []ChapterSummary
}

// GetTopic returns a topic and its chapters.
func (s *LibraryService) GetTopic(ctx context.Context, ownerID, id domain.ID) (TopicDetail, error) {
	t, err := s.Repos.Topics.Get(ctx, ownerID, id)
	if err != nil {
		return TopicDetail{}, err
	}
	chapters, err := s.Repos.Chapters.ListByTopic(ctx, ownerID, id)
	if err != nil {
		return TopicDetail{}, err
	}
	return TopicDetail{Topic: t, Chapters: chapters}, nil
}

// RenameTopic renames a topic.
func (s *LibraryService) RenameTopic(ctx context.Context, ownerID, id domain.ID, name string) (domain.Topic, error) {
	t, err := s.Repos.Topics.Get(ctx, ownerID, id)
	if err != nil {
		return domain.Topic{}, err
	}
	if err := t.Rename(name); err != nil {
		return domain.Topic{}, err
	}
	return t, s.Repos.Topics.Update(ctx, t)
}

// DeleteTopic deletes a topic and (by cascade) its chapters.
func (s *LibraryService) DeleteTopic(ctx context.Context, ownerID, id domain.ID) error {
	return s.Repos.Topics.Delete(ctx, ownerID, id)
}

// ChapterDetail is a chapter with its latest job and set size.
type ChapterDetail struct {
	Chapter       domain.Chapter
	Topic         domain.Topic
	LatestJob     *domain.ProcessingJob
	QuestionCount int
}

// GetChapter returns a chapter with its processing state.
func (s *LibraryService) GetChapter(ctx context.Context, ownerID, id domain.ID) (ChapterDetail, error) {
	c, err := s.Repos.Chapters.Get(ctx, ownerID, id)
	if err != nil {
		return ChapterDetail{}, err
	}
	t, err := s.Repos.Topics.Get(ctx, ownerID, c.TopicID)
	if err != nil {
		return ChapterDetail{}, err
	}
	d := ChapterDetail{Chapter: c, Topic: t}
	if j, err := s.Repos.Jobs.LatestForChapter(ctx, ownerID, id); err == nil {
		d.LatestJob = &j
	} else if !isNotFound(err) {
		return ChapterDetail{}, err
	}
	if c.Status == domain.ChapterReady {
		set, err := s.Repos.QuestionSets.GetByChapter(ctx, id)
		if err != nil {
			return ChapterDetail{}, err
		}
		d.QuestionCount = set.Size()
	}
	return d, nil
}

// RenameChapter renames a chapter.
func (s *LibraryService) RenameChapter(ctx context.Context, ownerID, id domain.ID, title string) (domain.Chapter, error) {
	c, err := s.Repos.Chapters.Get(ctx, ownerID, id)
	if err != nil {
		return domain.Chapter{}, err
	}
	if err := c.Rename(title); err != nil {
		return domain.Chapter{}, err
	}
	return c, s.Repos.Chapters.UpdateTitle(ctx, c)
}

// DeleteChapter deletes a chapter with its set, jobs and attempts.
func (s *LibraryService) DeleteChapter(ctx context.Context, ownerID, id domain.ID) error {
	return s.Repos.Chapters.Delete(ctx, ownerID, id)
}

// GetQuestionSet returns a ready chapter's full bank (with answers) for review.
func (s *LibraryService) GetQuestionSet(ctx context.Context, ownerID, chapterID domain.ID) (domain.QuestionSet, error) {
	c, err := s.Repos.Chapters.Get(ctx, ownerID, chapterID)
	if err != nil {
		return domain.QuestionSet{}, err
	}
	if c.Status != domain.ChapterReady {
		return domain.QuestionSet{}, domain.Invalid("chapter is not ready yet")
	}
	return s.Repos.QuestionSets.GetByChapter(ctx, chapterID)
}
