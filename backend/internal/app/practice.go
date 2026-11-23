package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

// PracticeService runs ChapterAttempts.
type PracticeService struct{ Deps }

// AttemptView is an attempt with its drawn questions in draw order.
type AttemptView struct {
	Attempt      domain.ChapterAttempt
	ChapterTitle string
	Questions    []domain.Question
}

// StartChapterAttempt draws count questions from a ready chapter.
func (s *PracticeService) StartChapterAttempt(ctx context.Context, ownerID, chapterID domain.ID, count int) (AttemptView, error) {
	ch, err := s.Repos.Chapters.Get(ctx, ownerID, chapterID)
	if err != nil {
		return AttemptView{}, err
	}
	if ch.Status != domain.ChapterReady {
		return AttemptView{}, domain.Invalid("chapter is not ready yet")
	}
	set, err := s.Repos.QuestionSets.GetByChapter(ctx, chapterID)
	if err != nil {
		return AttemptView{}, err
	}
	stats, err := s.Repos.Attempts.QuestionStats(ctx, ownerID, chapterID)
	if err != nil {
		return AttemptView{}, err
	}
	ids, err := domain.SelectQuestions(set, stats, count, s.rand())
	if err != nil {
		return AttemptView{}, err
	}
	a := domain.NewChapterAttempt(s.IDs.NewID(), ownerID, chapterID, ids, s.Clock.Now())
	if err := s.Repos.Attempts.Create(ctx, a); err != nil {
		return AttemptView{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "attempt.started", slog.String("attempt_id", a.ID.String()), slog.String("chapter_id", chapterID.String()), slog.Int("count", count))
	return view(a, ch, set), nil
}

// SubmitChapterAttempt grades and records responses.
func (s *PracticeService) SubmitChapterAttempt(ctx context.Context, ownerID, attemptID domain.ID, responses map[domain.ID][]int) (AttemptView, error) {
	a, err := s.Repos.Attempts.Get(ctx, ownerID, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	ch, err := s.Repos.Chapters.Get(ctx, ownerID, a.ChapterID)
	if err != nil {
		return AttemptView{}, err
	}
	set, err := s.Repos.QuestionSets.GetByChapter(ctx, a.ChapterID)
	if err != nil {
		return AttemptView{}, err
	}
	if err := a.Submit(set, responses, s.Clock.Now()); err != nil {
		return AttemptView{}, err
	}
	if err := s.Repos.Attempts.SaveSubmission(ctx, a); err != nil {
		return AttemptView{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "attempt.submitted", slog.String("attempt_id", a.ID.String()), slog.Float64("score", a.TotalScore), slog.Int("max_score", a.MaxScore))
	return view(a, ch, set), nil
}

// GetChapterAttempt returns one attempt.
func (s *PracticeService) GetChapterAttempt(ctx context.Context, ownerID, attemptID domain.ID) (AttemptView, error) {
	a, err := s.Repos.Attempts.Get(ctx, ownerID, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	ch, err := s.Repos.Chapters.Get(ctx, ownerID, a.ChapterID)
	if err != nil {
		return AttemptView{}, err
	}
	set, err := s.Repos.QuestionSets.GetByChapter(ctx, a.ChapterID)
	if err != nil {
		return AttemptView{}, err
	}
	return view(a, ch, set), nil
}

// ListChapterAttempts returns the chapter's attempt history, newest first.
func (s *PracticeService) ListChapterAttempts(ctx context.Context, ownerID, chapterID domain.ID) ([]domain.ChapterAttempt, error) {
	if _, err := s.Repos.Chapters.Get(ctx, ownerID, chapterID); err != nil {
		return nil, err
	}
	return s.Repos.Attempts.ListByChapter(ctx, ownerID, chapterID)
}

// AttemptHistoryLimit caps how many attempts ListAttempts returns.
const AttemptHistoryLimit = 200

// ListAttempts returns the user's most recent attempts across all chapters, newest first,
// capped at AttemptHistoryLimit.
func (s *PracticeService) ListAttempts(ctx context.Context, ownerID domain.ID) ([]AttemptSummary, error) {
	return s.Repos.Attempts.ListByOwner(ctx, ownerID, AttemptHistoryLimit)
}

func view(a domain.ChapterAttempt, ch domain.Chapter, set domain.QuestionSet) AttemptView {
	qs := make([]domain.Question, 0, len(a.QuestionIDs))
	for _, id := range a.QuestionIDs {
		if q, ok := set.Question(id); ok {
			qs = append(qs, q)
		}
	}
	return AttemptView{Attempt: a, ChapterTitle: ch.Title, Questions: qs}
}
