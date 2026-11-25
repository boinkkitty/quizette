package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

// PracticeService draws quizzes and records submitted ChapterAttempts.
type PracticeService struct{ Deps }

// AttemptView is an attempt with its drawn questions in draw order.
type AttemptView struct {
	Attempt      domain.ChapterAttempt
	ChapterTitle string
	Questions    []domain.Question
}

// DrawnQuiz is a fresh draw of questions for an unfinished quiz. Nothing is
// stored: the client keeps it until it submits (ADR 0011).
type DrawnQuiz struct {
	ChapterID    domain.ID
	ChapterTitle string
	Questions    []domain.Question
}

// DrawChapterQuiz draws count questions at random from a ready chapter.
func (s *PracticeService) DrawChapterQuiz(ctx context.Context, ownerID, chapterID domain.ID, count int) (DrawnQuiz, error) {
	ch, err := s.Repos.Chapters.Get(ctx, ownerID, chapterID)
	if err != nil {
		return DrawnQuiz{}, err
	}
	if ch.Status != domain.ChapterReady {
		return DrawnQuiz{}, domain.Invalid("chapter is not ready yet")
	}
	set, err := s.Repos.QuestionSets.GetByChapter(ctx, chapterID)
	if err != nil {
		return DrawnQuiz{}, err
	}
	ids, err := domain.DrawQuestions(set, count, s.rand())
	if err != nil {
		return DrawnQuiz{}, err
	}
	qs := make([]domain.Question, len(ids))
	for i, id := range ids {
		qs[i], _ = set.Question(id)
	}
	return DrawnQuiz{ChapterID: chapterID, ChapterTitle: ch.Title, Questions: qs}, nil
}

// SubmitChapterQuiz grades a finished quiz over questionIDs (as drawn, in draw order)
// and records it as an attempt. The questions must belong to the ready chapter and
// their count must pass the count rule; responses for other questions are rejected.
func (s *PracticeService) SubmitChapterQuiz(ctx context.Context, ownerID, chapterID domain.ID, questionIDs []domain.ID, responses map[domain.ID][]int) (AttemptView, error) {
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
	a, err := domain.NewChapterAttempt(s.IDs.NewID(), ownerID, set, questionIDs, responses, s.Clock.Now())
	if err != nil {
		return AttemptView{}, err
	}
	if err := s.Repos.Attempts.Create(ctx, a); err != nil {
		return AttemptView{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "attempt.submitted", slog.String("attempt_id", a.ID.String()), slog.String("chapter_id", chapterID.String()), slog.Float64("score", a.TotalScore), slog.Int("max_score", a.MaxScore))
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
