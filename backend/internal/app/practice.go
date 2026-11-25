package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

// PracticeService draws quizzes and records submitted Attempts.
type PracticeService struct{ Deps }

// ScopedQuestion is a question with the chapter it comes from, so a topic quiz's
// questions can be grouped by chapter.
type ScopedQuestion struct {
	domain.Question
	ChapterID    domain.ID
	ChapterTitle string
}

// AttemptView is an attempt with its drawn questions in draw order. ScopeName is the
// chapter's title or the topic's name, according to the attempt scope.
type AttemptView struct {
	Attempt   domain.Attempt
	ScopeName string
	Questions []ScopedQuestion
}

// DrawnQuiz is a fresh draw of questions for an unfinished quiz. Nothing is
// stored: the client keeps it until it submits (ADR 0011). ScopeName is the
// chapter's title or the topic's name, according to Scope.
type DrawnQuiz struct {
	Scope     domain.AttemptScope
	ScopeName string
	Questions []ScopedQuestion
}

// scopePool is what an attempt scope draws from, with the names to display.
type scopePool struct {
	name   string
	pool   domain.Pool
	titles map[domain.ID]string // chapter ID to title
}

// loadPool loads the question sets of the scope's ready chapters. A topic's
// chapters come in creation order, which is the order the draw splits them in.
// It fails with a validation error if the scope has no ready chapter.
func (s *PracticeService) loadPool(ctx context.Context, ownerID domain.ID, scope domain.AttemptScope) (scopePool, error) {
	var (
		name     string
		chapters []domain.Chapter
	)
	switch scope.Type {
	case domain.ScopeChapter:
		ch, err := s.Repos.Chapters.Get(ctx, ownerID, scope.ID)
		if err != nil {
			return scopePool{}, err
		}
		if ch.Status != domain.ChapterReady {
			return scopePool{}, domain.Invalid("chapter is not ready yet")
		}
		name, chapters = ch.Title, []domain.Chapter{ch}
	case domain.ScopeTopic:
		t, err := s.Repos.Topics.Get(ctx, ownerID, scope.ID)
		if err != nil {
			return scopePool{}, err
		}
		summaries, err := s.Repos.Chapters.ListByTopic(ctx, ownerID, scope.ID)
		if err != nil {
			return scopePool{}, err
		}
		for _, c := range summaries {
			if c.Status == domain.ChapterReady {
				chapters = append(chapters, c.Chapter)
			}
		}
		if len(chapters) == 0 {
			return scopePool{}, domain.Invalid("topic has no ready chapter")
		}
		name = t.Name
	default:
		return scopePool{}, domain.Invalid("scope must be a chapter or a topic")
	}
	out := scopePool{name: name, titles: make(map[domain.ID]string, len(chapters))}
	for _, ch := range chapters {
		set, err := s.Repos.QuestionSets.GetByChapter(ctx, ch.ID)
		if err != nil {
			return scopePool{}, err
		}
		out.pool = append(out.pool, set)
		out.titles[ch.ID] = ch.Title
	}
	return out, nil
}

// questions looks up ids in the pool, in order, skipping any it doesn't hold.
func (p scopePool) questions(ids []domain.ID) []ScopedQuestion {
	out := make([]ScopedQuestion, 0, len(ids))
	for _, id := range ids {
		q, ok := p.pool.Question(id)
		if !ok {
			continue
		}
		ch, _ := p.pool.ChapterOf(id)
		out = append(out, ScopedQuestion{Question: q, ChapterID: ch, ChapterTitle: p.titles[ch]})
	}
	return out
}

// DrawQuiz draws count questions at random from the scope's ready chapters,
// split across them by DrawQuestions.
func (s *PracticeService) DrawQuiz(ctx context.Context, ownerID domain.ID, scope domain.AttemptScope, count int) (DrawnQuiz, error) {
	p, err := s.loadPool(ctx, ownerID, scope)
	if err != nil {
		return DrawnQuiz{}, err
	}
	ids, err := domain.DrawQuestions(p.pool, count, s.rand())
	if err != nil {
		return DrawnQuiz{}, err
	}
	return DrawnQuiz{Scope: scope, ScopeName: p.name, Questions: p.questions(ids)}, nil
}

// SubmitQuiz grades a finished quiz over questionIDs (as drawn, in draw order) and
// records it as an attempt. The questions must belong to the scope's ready chapters
// and their count must pass the count rule; responses for other questions are rejected.
func (s *PracticeService) SubmitQuiz(ctx context.Context, ownerID domain.ID, scope domain.AttemptScope, questionIDs []domain.ID, responses map[domain.ID][]int) (AttemptView, error) {
	p, err := s.loadPool(ctx, ownerID, scope)
	if err != nil {
		return AttemptView{}, err
	}
	a, err := domain.NewAttempt(s.IDs.NewID(), ownerID, scope, p.pool, questionIDs, responses, s.Clock.Now())
	if err != nil {
		return AttemptView{}, err
	}
	if err := s.Repos.Attempts.Create(ctx, a); err != nil {
		return AttemptView{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "attempt.submitted", slog.String("attempt_id", a.ID.String()),
		slog.String("scope", string(scope.Type)), slog.String("scope_id", scope.ID.String()),
		slog.Float64("score", a.TotalScore), slog.Int("max_score", a.MaxScore))
	return AttemptView{Attempt: a, ScopeName: p.name, Questions: p.questions(a.QuestionIDs)}, nil
}

// GetAttempt returns one attempt.
func (s *PracticeService) GetAttempt(ctx context.Context, ownerID, attemptID domain.ID) (AttemptView, error) {
	a, err := s.Repos.Attempts.Get(ctx, ownerID, attemptID)
	if err != nil {
		return AttemptView{}, err
	}
	p, err := s.loadPool(ctx, ownerID, a.Scope)
	if err != nil {
		return AttemptView{}, err
	}
	return AttemptView{Attempt: a, ScopeName: p.name, Questions: p.questions(a.QuestionIDs)}, nil
}

// ListChapterAttempts returns the chapter's chapter-scoped attempt history, newest first.
func (s *PracticeService) ListChapterAttempts(ctx context.Context, ownerID, chapterID domain.ID) ([]domain.Attempt, error) {
	if _, err := s.Repos.Chapters.Get(ctx, ownerID, chapterID); err != nil {
		return nil, err
	}
	return s.Repos.Attempts.ListByChapter(ctx, ownerID, chapterID)
}

// AttemptHistoryLimit caps how many attempts ListAttempts returns.
const AttemptHistoryLimit = 200

// ListAttempts returns the user's most recent attempts across all scopes, newest first,
// capped at AttemptHistoryLimit.
func (s *PracticeService) ListAttempts(ctx context.Context, ownerID domain.ID) ([]AttemptSummary, error) {
	return s.Repos.Attempts.ListByOwner(ctx, ownerID, AttemptHistoryLimit)
}
