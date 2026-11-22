//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/stub"
	"github.com/boinkkitty/quizette/backend/internal/adapters/postgres"
	"github.com/boinkkitty/quizette/backend/internal/adapters/system"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/app/apptest"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// openDB uses TEST_DATABASE_URL if set (CI service container), otherwise
// starts a throwaway postgres:16 container.
func openDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("quizette"), tcpostgres.WithUsername("quizette"), tcpostgres.WithPassword("quizette"),
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)))
		if err != nil {
			t.Fatalf("start postgres: %v", err)
		}
		t.Cleanup(func() { _ = ctr.Terminate(ctx) })
		url, err = ctr.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
	}
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestRepositoriesFullFlow(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	deps := app.Deps{Repos: store.Repos(), UoW: store, Clock: system.Clock{}, IDs: system.UUIDv7{}, Logger: slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))}
	queue := &apptest.SyncQueue{}
	gen := &app.GenerationService{Deps: deps, Generator: stub.Generator{}, Queue: queue, Config: app.GenerationConfig{}}
	queue.Handler = gen.ProcessJob
	ids := &app.IdentityService{Deps: deps, Hasher: system.Bcrypt{Cost: 4}}
	lib := &app.LibraryService{Deps: deps}
	prac := &app.PracticeService{Deps: deps}

	u, tok, err := ids.SignUp(ctx, "Ada@Example.com", "correct horse", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ids.SignUp(ctx, "ADA@example.com", "correct horse", ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate email: %v", err)
	}
	if got, err := ids.Authenticate(ctx, string(tok.Token)); err != nil || got.ID != u.ID {
		t.Fatalf("authenticate: %v", err)
	}
	if err := ids.LogOut(ctx, string(tok.Token)); err != nil {
		t.Fatal(err)
	}
	if _, err := ids.Authenticate(ctx, string(tok.Token)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("after logout: %v", err)
	}

	res, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, NewTopicName: "Biology", Count: 10, Filename: "cells.pdf", Data: []byte("%PDF-1.4 test")})
	if err != nil {
		t.Fatal(err)
	}
	job, err := gen.GetJob(ctx, u.ID, res.JobID)
	if err != nil || job.Status != domain.JobSucceeded || job.Metrics.TotalTokens == 0 || job.Document.SHA256 == "" {
		t.Fatalf("job: %v %+v", err, job)
	}
	if _, err := lib.CreateTopic(ctx, u.ID, "biology"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("case-insensitive topic uniqueness: %v", err)
	}

	set, err := lib.GetQuestionSet(ctx, u.ID, res.ChapterID)
	if err != nil || set.Size() != 10 {
		t.Fatalf("set: %v size %d", err, set.Size())
	}

	v, err := prac.StartChapterAttempt(ctx, u.ID, res.ChapterID, 5)
	if err != nil {
		t.Fatal(err)
	}
	responses := map[domain.ID][]int{}
	for _, q := range v.Questions {
		responses[q.ID] = q.Correct
	}
	responses[v.Questions[0].ID] = []int{} // one wrong
	if _, err := prac.SubmitChapterAttempt(ctx, u.ID, v.Attempt.ID, responses); err != nil {
		t.Fatal(err)
	}
	got, err := prac.GetChapterAttempt(ctx, u.ID, v.Attempt.ID)
	if err != nil || got.Attempt.TotalScore != 4 || len(got.Attempt.Answers) != 5 || got.Attempt.SubmittedAt == nil {
		t.Fatalf("attempt: %v %+v", err, got.Attempt)
	}

	stats, err := store.Repos().Attempts.QuestionStats(ctx, u.ID, res.ChapterID)
	if err != nil || len(stats) != 5 || stats[v.Questions[0].ID].LastPoints != 0 {
		t.Fatalf("stats: %v %+v", err, stats)
	}

	detail, err := lib.GetTopic(ctx, u.ID, res.TopicID)
	if err != nil || len(detail.Chapters) != 1 || detail.Chapters[0].AttemptCount != 1 || detail.Chapters[0].BestScorePct == nil || *detail.Chapters[0].BestScorePct != 80 {
		t.Fatalf("topic detail: %v %+v", err, detail.Chapters)
	}

	all, err := lib.ListChapters(ctx, u.ID, app.ChapterFilter{})
	if err != nil || len(all) != 1 || all[0].TopicName != "Biology" || all[0].QuestionCount != 10 || all[0].BestScorePct == nil {
		t.Fatalf("all chapters: %v %+v", err, all)
	}
	for _, tc := range []struct {
		filter app.ChapterFilter
		want   int
	}{
		{app.ChapterFilter{Status: domain.ChapterReady}, 1},
		{app.ChapterFilter{Status: domain.ChapterPending}, 0},
		{app.ChapterFilter{TopicID: res.TopicID, Status: domain.ChapterReady}, 1},
		{app.ChapterFilter{TopicID: "0190f5e2-0000-7000-8000-000000000000"}, 0},
	} {
		got, err := lib.ListChapters(ctx, u.ID, tc.filter)
		if err != nil || len(got) != tc.want {
			t.Errorf("ListChapters(%+v) = %d chapters, %v; want %d", tc.filter, len(got), err, tc.want)
		}
	}
	// A second, unsubmitted attempt: history is newest first and the limit applies.
	v2, err := prac.StartChapterAttempt(ctx, u.ID, res.ChapterID, 5)
	if err != nil {
		t.Fatal(err)
	}
	hist, err := prac.ListAttempts(ctx, u.ID)
	if err != nil || len(hist) != 2 || hist[0].ID != v2.Attempt.ID || hist[1].ID != v.Attempt.ID ||
		hist[1].ChapterTitle != "cells" || hist[1].TopicName != "Biology" || hist[1].TopicID != res.TopicID || hist[1].TotalScore != 4 {
		t.Fatalf("attempt history: %v %+v", err, hist)
	}
	if limited, err := store.Repos().Attempts.ListByOwner(ctx, u.ID, 1); err != nil || len(limited) != 1 || limited[0].ID != v2.Attempt.ID {
		t.Fatalf("limited history: %v %+v", err, limited)
	}

	// Ownership is enforced at query level.
	if foreign, err := prac.ListAttempts(ctx, "00000000-0000-7000-8000-000000000000"); err != nil || len(foreign) != 0 {
		t.Fatalf("foreign history: %v %+v", err, foreign)
	}
	if _, err := lib.GetChapter(ctx, "00000000-0000-7000-8000-000000000000", res.ChapterID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign owner: %v", err)
	}
	if _, err := lib.GetChapter(ctx, u.ID, "not-a-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("malformed id: %v", err)
	}

	// Recovery fails unfinished jobs.
	queue.Handler = nil
	res2, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, TopicID: res.TopicID, Count: 5, Filename: "b.pdf", Data: []byte("%PDF-1.4 b")})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := gen.RecoverInterrupted(ctx); err != nil || n != 1 {
		t.Fatalf("recover: %d %v", n, err)
	}
	if ch, _ := lib.GetChapter(ctx, u.ID, res2.ChapterID); ch.Chapter.Status != domain.ChapterFailed {
		t.Fatalf("chapter after recovery: %s", ch.Chapter.Status)
	}

	// Cascade delete removes the chapter's set, jobs and attempts.
	if err := lib.DeleteTopic(ctx, u.ID, res.TopicID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM questions").Scan(&n); err != nil || n != 0 {
		t.Fatalf("questions after cascade: %d %v", n, err)
	}
}

func TestNoDocumentBytesColumn(t *testing.T) {
	pool := openDB(t)
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND data_type='bytea'`).Scan(&n)
	if err != nil || n != 0 {
		t.Fatalf("found %d bytea columns (documents must never be stored): %v", n, err)
	}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }
