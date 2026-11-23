package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/postgres/sqlcgen"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// ---- topics ----

type topicRepo struct{ q *sqlcgen.Queries }

func toTopic(r sqlcgen.Topic) domain.Topic {
	return domain.Topic{ID: domain.ID(r.ID), OwnerID: domain.ID(r.OwnerID), Name: r.Name, CreatedAt: r.CreatedAt}
}

func (r topicRepo) Create(ctx context.Context, t domain.Topic) error {
	return mapErr("topic", r.q.CreateTopic(ctx, sqlcgen.CreateTopicParams{ID: string(t.ID), OwnerID: string(t.OwnerID), Name: t.Name, CreatedAt: t.CreatedAt}))
}

func (r topicRepo) Get(ctx context.Context, owner, id domain.ID) (domain.Topic, error) {
	row, err := r.q.GetTopic(ctx, sqlcgen.GetTopicParams{OwnerID: string(owner), ID: string(id)})
	return toTopic(row), mapErr("topic", err)
}

func (r topicRepo) GetByName(ctx context.Context, owner domain.ID, name string) (domain.Topic, error) {
	row, err := r.q.GetTopicByName(ctx, sqlcgen.GetTopicByNameParams{OwnerID: string(owner), Name: name})
	return toTopic(row), mapErr("topic", err)
}

func (r topicRepo) List(ctx context.Context, owner domain.ID) ([]app.TopicSummary, error) {
	rows, err := r.q.ListTopics(ctx, string(owner))
	if err != nil {
		return nil, mapErr("topics", err)
	}
	out := make([]app.TopicSummary, len(rows))
	for i, row := range rows {
		out[i] = app.TopicSummary{
			Topic:        domain.Topic{ID: domain.ID(row.ID), OwnerID: domain.ID(row.OwnerID), Name: row.Name, CreatedAt: row.CreatedAt},
			ChapterCount: int(row.ChapterCount),
		}
	}
	return out, nil
}

func (r topicRepo) Update(ctx context.Context, t domain.Topic) error {
	n, err := r.q.UpdateTopic(ctx, sqlcgen.UpdateTopicParams{OwnerID: string(t.OwnerID), ID: string(t.ID), Name: t.Name})
	return mustAffect("topic", n, err)
}

func (r topicRepo) Delete(ctx context.Context, owner, id domain.ID) error {
	n, err := r.q.DeleteTopic(ctx, sqlcgen.DeleteTopicParams{OwnerID: string(owner), ID: string(id)})
	return mustAffect("topic", n, err)
}

// ---- chapters ----

type chapterRepo struct{ q *sqlcgen.Queries }

func toChapter(r sqlcgen.Chapter) domain.Chapter {
	return domain.Chapter{ID: domain.ID(r.ID), TopicID: domain.ID(r.TopicID), OwnerID: domain.ID(r.OwnerID), Title: r.Title, Status: domain.ChapterStatus(r.Status), CreatedAt: r.CreatedAt}
}

func (r chapterRepo) Create(ctx context.Context, c domain.Chapter) error {
	return mapErr("chapter", r.q.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ID: string(c.ID), TopicID: string(c.TopicID), OwnerID: string(c.OwnerID), Title: c.Title, Status: string(c.Status), CreatedAt: c.CreatedAt,
	}))
}

func (r chapterRepo) Get(ctx context.Context, owner, id domain.ID) (domain.Chapter, error) {
	row, err := r.q.GetChapter(ctx, sqlcgen.GetChapterParams{OwnerID: string(owner), ID: string(id)})
	return toChapter(row), mapErr("chapter", err)
}

func (r chapterRepo) GetByID(ctx context.Context, id domain.ID) (domain.Chapter, error) {
	row, err := r.q.GetChapterByID(ctx, string(id))
	return toChapter(row), mapErr("chapter", err)
}

func (r chapterRepo) ListByTopic(ctx context.Context, owner, topicID domain.ID) ([]app.ChapterSummary, error) {
	rows, err := r.q.ListChaptersByTopic(ctx, sqlcgen.ListChaptersByTopicParams{OwnerID: string(owner), TopicID: string(topicID)})
	if err != nil {
		return nil, mapErr("chapters", err)
	}
	out := make([]app.ChapterSummary, len(rows))
	for i, row := range rows {
		out[i] = toChapterSummary(row)
	}
	return out, nil
}

func (r chapterRepo) ListByOwner(ctx context.Context, owner domain.ID, filter app.ChapterFilter) ([]app.ChapterSummary, error) {
	params := sqlcgen.ListChaptersByOwnerParams{OwnerID: string(owner)}
	if filter.TopicID != "" {
		params.TopicID = new(string(filter.TopicID))
	}
	if filter.Status != "" {
		params.Status = new(string(filter.Status))
	}
	rows, err := r.q.ListChaptersByOwner(ctx, params)
	if err != nil {
		return nil, mapErr("chapters", err)
	}
	out := make([]app.ChapterSummary, len(rows))
	for i, row := range rows {
		// Both queries select the same columns, so sqlc's row types are convertible.
		out[i] = toChapterSummary(sqlcgen.ListChaptersByTopicRow(row))
	}
	return out, nil
}

func toChapterSummary(row sqlcgen.ListChaptersByTopicRow) app.ChapterSummary {
	cs := app.ChapterSummary{
		Chapter:       domain.Chapter{ID: domain.ID(row.ID), TopicID: domain.ID(row.TopicID), OwnerID: domain.ID(row.OwnerID), Title: row.Title, Status: domain.ChapterStatus(row.Status), CreatedAt: row.CreatedAt},
		TopicName:     row.TopicName,
		QuestionCount: int(row.QuestionCount),
		AttemptCount:  int(row.AttemptCount),
	}
	if row.BestScorePct >= 0 {
		v := row.BestScorePct
		cs.BestScorePct = &v
	}
	return cs
}

func (r chapterRepo) UpdateTitle(ctx context.Context, c domain.Chapter) error {
	n, err := r.q.UpdateChapterTitle(ctx, sqlcgen.UpdateChapterTitleParams{OwnerID: string(c.OwnerID), ID: string(c.ID), Title: c.Title})
	return mustAffect("chapter", n, err)
}

func (r chapterRepo) UpdateStatus(ctx context.Context, c domain.Chapter) error {
	n, err := r.q.UpdateChapterStatus(ctx, sqlcgen.UpdateChapterStatusParams{ID: string(c.ID), Status: string(c.Status)})
	return mustAffect("chapter", n, err)
}

func (r chapterRepo) Delete(ctx context.Context, owner, id domain.ID) error {
	n, err := r.q.DeleteChapter(ctx, sqlcgen.DeleteChapterParams{OwnerID: string(owner), ID: string(id)})
	return mustAffect("chapter", n, err)
}

// ---- processing jobs ----

type jobRepo struct{ q *sqlcgen.Queries }

func toJob(r sqlcgen.ProcessingJob) domain.ProcessingJob {
	mix := make([]domain.QuestionType, len(r.TypeMix))
	for i, t := range r.TypeMix {
		mix[i] = domain.QuestionType(t)
	}
	return domain.ProcessingJob{
		ID: domain.ID(r.ID), ChapterID: domain.ID(r.ChapterID), OwnerID: domain.ID(r.OwnerID),
		Spec:     domain.QuestionSpec{Count: int(r.RequestedCount), TypeMix: mix},
		Document: domain.DocumentMeta{Filename: r.DocFilename, Size: r.DocSize, SHA256: r.DocSha256},
		Status:   domain.JobStatus(r.Status), Attempts: int(r.Attempts), Error: r.Error,
		Metrics: domain.GenerationMetrics{
			Strategy: r.Strategy, Model: r.Model,
			InputTokens: int(r.InputTokens), OutputTokens: int(r.OutputTokens), ThinkingTokens: int(r.ThinkingTokens), TotalTokens: int(r.TotalTokens),
			ExtractMS: r.ExtractMs, LLMMS: r.LlmMs, TotalMS: r.TotalMs, ExtractedChars: int(r.ExtractedChars), ValidationRetries: int(r.ValidationRetries),
		},
		QueuedAt: r.QueuedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

func (r jobRepo) Create(ctx context.Context, j domain.ProcessingJob) error {
	mix := make([]string, len(j.Spec.TypeMix))
	for i, t := range j.Spec.TypeMix {
		mix[i] = string(t)
	}
	return mapErr("job", r.q.CreateJob(ctx, sqlcgen.CreateJobParams{
		ID: string(j.ID), ChapterID: string(j.ChapterID), OwnerID: string(j.OwnerID),
		RequestedCount: int32(j.Spec.Count), TypeMix: mix, //nolint:gosec // bounded by domain.MaxSetSize
		DocFilename: j.Document.Filename, DocSize: j.Document.Size, DocSha256: j.Document.SHA256,
		Status: string(j.Status), Attempts: int32(j.Attempts), Error: j.Error, QueuedAt: j.QueuedAt, //nolint:gosec // small counter
	}))
}

func (r jobRepo) Get(ctx context.Context, owner, id domain.ID) (domain.ProcessingJob, error) {
	row, err := r.q.GetJob(ctx, sqlcgen.GetJobParams{OwnerID: string(owner), ID: string(id)})
	return toJob(row), mapErr("job", err)
}

func (r jobRepo) GetByID(ctx context.Context, id domain.ID) (domain.ProcessingJob, error) {
	row, err := r.q.GetJobByID(ctx, string(id))
	return toJob(row), mapErr("job", err)
}

func (r jobRepo) LatestForChapter(ctx context.Context, owner, chapterID domain.ID) (domain.ProcessingJob, error) {
	row, err := r.q.LatestJobForChapter(ctx, sqlcgen.LatestJobForChapterParams{OwnerID: string(owner), ChapterID: string(chapterID)})
	return toJob(row), mapErr("job", err)
}

func (r jobRepo) Update(ctx context.Context, j domain.ProcessingJob) error {
	m := j.Metrics
	n, err := r.q.UpdateJob(ctx, sqlcgen.UpdateJobParams{
		ID: string(j.ID), Status: string(j.Status), Attempts: i32(j.Attempts), Error: j.Error,
		Strategy: m.Strategy, Model: m.Model,
		InputTokens: i32(m.InputTokens), OutputTokens: i32(m.OutputTokens), ThinkingTokens: i32(m.ThinkingTokens), TotalTokens: i32(m.TotalTokens),
		ExtractMs: m.ExtractMS, LlmMs: m.LLMMS, TotalMs: m.TotalMS, ExtractedChars: i32(m.ExtractedChars), ValidationRetries: i32(m.ValidationRetries),
		StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	})
	return mustAffect("job", n, err)
}

func (r jobRepo) ListUnfinished(ctx context.Context) ([]domain.ProcessingJob, error) {
	rows, err := r.q.ListUnfinishedJobs(ctx)
	if err != nil {
		return nil, mapErr("jobs", err)
	}
	out := make([]domain.ProcessingJob, len(rows))
	for i, row := range rows {
		out[i] = toJob(row)
	}
	return out, nil
}

// ---- question sets ----

type setRepo struct {
	q  *sqlcgen.Queries
	db beginner
}

func (r setRepo) Create(ctx context.Context, s domain.QuestionSet) error {
	return inTx(ctx, r.db, func(q *sqlcgen.Queries) error {
		if err := q.CreateQuestionSet(ctx, sqlcgen.CreateQuestionSetParams{ID: string(s.ID), ChapterID: string(s.ChapterID), CreatedAt: s.CreatedAt}); err != nil {
			return mapErr("question set", err)
		}
		for _, qu := range s.Questions {
			opts, err := json.Marshal(qu.Options)
			if err != nil {
				return fmt.Errorf("marshal options: %w", err)
			}
			err = q.CreateQuestion(ctx, sqlcgen.CreateQuestionParams{
				ID: string(qu.ID), QuestionSetID: string(s.ID), Position: i32(qu.Position), Type: string(qu.Type),
				Prompt: qu.Prompt, Options: string(opts), Correct: ints32(qu.Correct), Explanation: qu.Explanation, SourceHint: qu.SourceHint,
			})
			if err != nil {
				return mapErr("question", err)
			}
		}
		return nil
	})
}

func (r setRepo) GetByChapter(ctx context.Context, chapterID domain.ID) (domain.QuestionSet, error) {
	row, err := r.q.GetQuestionSetByChapter(ctx, string(chapterID))
	if err != nil {
		return domain.QuestionSet{}, mapErr("question set", err)
	}
	qs, err := r.q.ListQuestionsBySet(ctx, row.ID)
	if err != nil {
		return domain.QuestionSet{}, mapErr("questions", err)
	}
	set := domain.QuestionSet{ID: domain.ID(row.ID), ChapterID: domain.ID(row.ChapterID), CreatedAt: row.CreatedAt, Questions: make([]domain.Question, len(qs))}
	for i, q := range qs {
		var opts []string
		if err := json.Unmarshal([]byte(q.Options), &opts); err != nil {
			return domain.QuestionSet{}, fmt.Errorf("decode options of question %s: %w", q.ID, err)
		}
		set.Questions[i] = domain.Question{
			ID: domain.ID(q.ID), Position: int(q.Position), Type: domain.QuestionType(q.Type), Prompt: q.Prompt,
			Options: opts, Correct: ints(q.Correct), Explanation: q.Explanation, SourceHint: q.SourceHint,
		}
	}
	return set, nil
}

// ---- attempts ----

type attemptRepo struct {
	q  *sqlcgen.Queries
	db beginner
}

func toAttempt(r sqlcgen.ChapterAttempt) domain.ChapterAttempt {
	ids := make([]domain.ID, len(r.QuestionIds))
	for i, id := range r.QuestionIds {
		ids[i] = domain.ID(id)
	}
	return domain.ChapterAttempt{
		ID: domain.ID(r.ID), OwnerID: domain.ID(r.OwnerID), ChapterID: domain.ID(r.ChapterID),
		RequestedCount: int(r.RequestedCount), QuestionIDs: ids, Status: domain.AttemptStatus(r.Status),
		TotalScore: r.TotalScore, MaxScore: int(r.MaxScore), StartedAt: r.StartedAt, SubmittedAt: r.SubmittedAt,
	}
}

func (r attemptRepo) Create(ctx context.Context, a domain.ChapterAttempt) error {
	ids := make([]string, len(a.QuestionIDs))
	for i, id := range a.QuestionIDs {
		ids[i] = string(id)
	}
	return mapErr("attempt", r.q.CreateAttempt(ctx, sqlcgen.CreateAttemptParams{
		ID: string(a.ID), OwnerID: string(a.OwnerID), ChapterID: string(a.ChapterID), RequestedCount: i32(a.RequestedCount),
		QuestionIds: ids, Status: string(a.Status), TotalScore: a.TotalScore, MaxScore: i32(a.MaxScore), StartedAt: a.StartedAt,
	}))
}

func (r attemptRepo) Get(ctx context.Context, owner, id domain.ID) (domain.ChapterAttempt, error) {
	row, err := r.q.GetAttempt(ctx, sqlcgen.GetAttemptParams{OwnerID: string(owner), ID: string(id)})
	if err != nil {
		return domain.ChapterAttempt{}, mapErr("attempt", err)
	}
	a := toAttempt(row)
	answers, err := r.q.ListAnswers(ctx, row.ID)
	if err != nil {
		return domain.ChapterAttempt{}, mapErr("answers", err)
	}
	a.Answers = make([]domain.Answer, len(answers))
	for i, ans := range answers {
		a.Answers[i] = domain.Answer{QuestionID: domain.ID(ans.QuestionID), Position: int(ans.Position), Selected: ints(ans.Selected), Score: domain.Score{Points: ans.Points, Correct: ans.Correct}}
	}
	return a, nil
}

func (r attemptRepo) SaveSubmission(ctx context.Context, a domain.ChapterAttempt) error {
	return inTx(ctx, r.db, func(q *sqlcgen.Queries) error {
		n, err := q.SubmitAttempt(ctx, sqlcgen.SubmitAttemptParams{ID: string(a.ID), TotalScore: a.TotalScore, SubmittedAt: a.SubmittedAt})
		if err != nil {
			return mapErr("attempt", err)
		}
		if n == 0 {
			// Already submitted (another request got there first) or deleted meanwhile.
			return fmt.Errorf("attempt already submitted: %w", domain.ErrConflict)
		}
		if err := q.DeleteAnswers(ctx, string(a.ID)); err != nil {
			return mapErr("answers", err)
		}
		for _, ans := range a.Answers {
			err := q.InsertAnswer(ctx, sqlcgen.InsertAnswerParams{
				AttemptID: string(a.ID), QuestionID: string(ans.QuestionID), Position: i32(ans.Position),
				Selected: ints32(ans.Selected), Points: ans.Score.Points, Correct: ans.Score.Correct,
			})
			if err != nil {
				return mapErr("answer", err)
			}
		}
		return nil
	})
}

func (r attemptRepo) ListByChapter(ctx context.Context, owner, chapterID domain.ID) ([]domain.ChapterAttempt, error) {
	rows, err := r.q.ListAttemptsByChapter(ctx, sqlcgen.ListAttemptsByChapterParams{OwnerID: string(owner), ChapterID: string(chapterID)})
	if err != nil {
		return nil, mapErr("attempts", err)
	}
	out := make([]domain.ChapterAttempt, len(rows))
	for i, row := range rows {
		out[i] = toAttempt(row)
	}
	return out, nil
}

func (r attemptRepo) ListByOwner(ctx context.Context, owner domain.ID, limit int) ([]app.AttemptSummary, error) {
	rows, err := r.q.ListAttemptsByOwner(ctx, sqlcgen.ListAttemptsByOwnerParams{OwnerID: string(owner), Limit: i32(limit)})
	if err != nil {
		return nil, mapErr("attempts", err)
	}
	out := make([]app.AttemptSummary, len(rows))
	for i, row := range rows {
		out[i] = app.AttemptSummary{
			ChapterAttempt: toAttempt(sqlcgen.ChapterAttempt{
				ID: row.ID, OwnerID: row.OwnerID, ChapterID: row.ChapterID, RequestedCount: row.RequestedCount, QuestionIds: row.QuestionIds,
				Status: row.Status, TotalScore: row.TotalScore, MaxScore: row.MaxScore, StartedAt: row.StartedAt, SubmittedAt: row.SubmittedAt,
			}),
			ChapterTitle: row.ChapterTitle,
			TopicID:      domain.ID(row.TopicID),
			TopicName:    row.TopicName,
		}
	}
	return out, nil
}

func (r attemptRepo) QuestionStats(ctx context.Context, owner, chapterID domain.ID) (map[domain.ID]domain.QuestionStat, error) {
	rows, err := r.q.QuestionStats(ctx, sqlcgen.QuestionStatsParams{OwnerID: string(owner), ChapterID: string(chapterID)})
	if err != nil {
		return nil, mapErr("question stats", err)
	}
	out := make(map[domain.ID]domain.QuestionStat, len(rows))
	for _, row := range rows {
		out[domain.ID(row.QuestionID)] = domain.QuestionStat{Attempts: int(row.Attempts), LastPoints: row.LastPoints}
	}
	return out, nil
}

// ---- users & sessions ----

type userRepo struct{ q *sqlcgen.Queries }

func toUser(r sqlcgen.User) domain.User {
	u := domain.User{ID: domain.ID(r.ID), Email: r.Email, Name: r.Name, AvatarURL: r.AvatarUrl, CreatedAt: r.CreatedAt}
	if r.PasswordHash != nil {
		u.PasswordHash = *r.PasswordHash
	}
	return u
}

func (r userRepo) Create(ctx context.Context, u domain.User) error {
	var hash *string
	if u.PasswordHash != "" {
		hash = &u.PasswordHash
	}
	return mapErr("user", r.q.CreateUser(ctx, sqlcgen.CreateUserParams{ID: string(u.ID), Email: u.Email, PasswordHash: hash, Name: u.Name, AvatarUrl: u.AvatarURL, CreatedAt: u.CreatedAt}))
}

func (r userRepo) GetByID(ctx context.Context, id domain.ID) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, string(id))
	return toUser(row), mapErr("user", err)
}

func (r userRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	return toUser(row), mapErr("user", err)
}

func (r userRepo) GetByOAuth(ctx context.Context, provider, puid string) (domain.User, error) {
	row, err := r.q.GetUserByOAuth(ctx, sqlcgen.GetUserByOAuthParams{Provider: provider, ProviderUserID: puid})
	return toUser(row), mapErr("user", err)
}

func (r userRepo) LinkOAuth(ctx context.Context, userID domain.ID, provider, puid string) error {
	return mapErr("oauth account", r.q.LinkOAuth(ctx, sqlcgen.LinkOAuthParams{UserID: string(userID), Provider: provider, ProviderUserID: puid}))
}

type sessionRepo struct{ q *sqlcgen.Queries }

func (r sessionRepo) Create(ctx context.Context, s app.Session) error {
	return mapErr("session", r.q.CreateSession(ctx, sqlcgen.CreateSessionParams{TokenHash: s.TokenHash, UserID: string(s.UserID), ExpiresAt: s.ExpiresAt, CreatedAt: s.CreatedAt}))
}

func (r sessionRepo) Get(ctx context.Context, hash string) (app.Session, error) {
	row, err := r.q.GetSession(ctx, hash)
	return app.Session{TokenHash: row.TokenHash, UserID: domain.ID(row.UserID), ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt}, mapErr("session", err)
}

func (r sessionRepo) Delete(ctx context.Context, hash string) error {
	return mapErr("session", r.q.DeleteSession(ctx, hash))
}

func (r sessionRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	n, err := r.q.DeleteExpiredSessions(ctx, now)
	return n, mapErr("sessions", err)
}

// ---- helpers ----

func i32(n int) int32 { return int32(n) } //nolint:gosec // all callers pass small, domain-bounded values

func ints32(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = i32(v)
	}
	return out
}

func ints(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
