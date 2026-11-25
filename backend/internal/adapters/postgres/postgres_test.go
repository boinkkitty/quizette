//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/stub"
	"github.com/boinkkitty/quizette/backend/internal/adapters/postgres"
	"github.com/boinkkitty/quizette/backend/internal/adapters/system"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/app/apptest"
	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/migrations"
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

	quiz, err := prac.DrawChapterQuiz(ctx, u.ID, res.ChapterID, 5)
	if err != nil {
		t.Fatal(err)
	}
	drawn := make([]domain.ID, len(quiz.Questions))
	responses := map[domain.ID][]int{}
	for i, q := range quiz.Questions {
		drawn[i] = q.ID
		responses[q.ID] = q.Correct
	}
	responses[drawn[0]] = []int{} // one wrong
	v, err := prac.SubmitChapterQuiz(ctx, u.ID, res.ChapterID, drawn, responses)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prac.GetChapterAttempt(ctx, u.ID, v.Attempt.ID)
	if err != nil || got.Attempt.TotalScore != 4 || len(got.Attempt.Answers) != 5 || got.Attempt.SubmittedAt.IsZero() ||
		!slices.Equal(got.Attempt.QuestionIDs, drawn) || got.Attempt.Answers[0].QuestionID != drawn[0] || got.Attempt.Answers[0].Score.Points != 0 {
		t.Fatalf("attempt: %v %+v", err, got.Attempt)
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
	// A second attempt: history is newest first and the limit applies.
	time.Sleep(time.Millisecond) // started_at orders the history
	v2, err := prac.SubmitChapterQuiz(ctx, u.ID, res.ChapterID, drawn, nil)
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

// newServices wires the app services to a real database for the race tests below.
// With a nil queue handler, uploads stay pending until the test finishes them.
func newServices(t *testing.T, pool *pgxpool.Pool, process bool) (*postgres.Store, *app.GenerationService, *app.LibraryService, *app.PracticeService, domain.User) {
	t.Helper()
	store := postgres.NewStore(pool)
	deps := app.Deps{Repos: store.Repos(), UoW: store, Clock: system.Clock{}, IDs: system.UUIDv7{}, Logger: slog.New(slog.DiscardHandler)}
	queue := &apptest.SyncQueue{}
	gen := &app.GenerationService{Deps: deps, Generator: stub.Generator{}, Queue: queue}
	if process {
		queue.Handler = gen.ProcessJob
	}
	ids := &app.IdentityService{Deps: deps, Hasher: system.Bcrypt{Cost: 4}}
	// Unique per run: CI shares one database between tests.
	u, _, err := ids.SignUp(context.Background(), string(system.UUIDv7{}.NewID())+"@example.com", "correct horse", "")
	if err != nil {
		t.Fatal(err)
	}
	return store, gen, &app.LibraryService{Deps: deps}, &app.PracticeService{Deps: deps}, u
}

// Regression: renaming a chapter while its questions were being generated saved the
// whole row from a stale copy, so one change overwrote the other. Either a ready
// chapter went back to pending (questions unreachable) or the new title was lost.
func TestChapterRenameDuringGeneration(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	store, gen, lib, _, u := newServices(t, pool, false)
	chapters := store.Repos().Chapters
	upload := func(name string) app.UploadResult {
		res, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, NewTopicName: name, Count: 5, Filename: "a.pdf", Data: []byte("%PDF-1.4 a")})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = lib.DeleteTopic(ctx, u.ID, res.TopicID) })
		return res
	}

	// The rename reads the chapter while it's pending; the job marks it ready before
	// the rename saves.
	a := upload("Rename saves last")
	staleForRename, _ := chapters.Get(ctx, u.ID, a.ChapterID)
	ready, _ := chapters.GetByID(ctx, a.ChapterID)
	if err := ready.MarkReady(); err != nil {
		t.Fatal(err)
	}
	if err := chapters.UpdateStatus(ctx, ready); err != nil {
		t.Fatal(err)
	}
	if err := staleForRename.Rename("Renamed"); err != nil {
		t.Fatal(err)
	}
	if err := chapters.UpdateTitle(ctx, staleForRename); err != nil {
		t.Fatal(err)
	}
	if got, _ := chapters.GetByID(ctx, a.ChapterID); got.Status != domain.ChapterReady || got.Title != "Renamed" {
		t.Errorf("rename saved last: got %s %q, want ready %q", got.Status, got.Title, "Renamed")
	}

	// The job reads the chapter, the user renames it, then the job saves its status.
	b := upload("Job saves last")
	staleForJob, _ := chapters.GetByID(ctx, b.ChapterID)
	if _, err := lib.RenameChapter(ctx, u.ID, b.ChapterID, "Renamed too"); err != nil {
		t.Fatal(err)
	}
	if err := staleForJob.MarkReady(); err != nil {
		t.Fatal(err)
	}
	if err := chapters.UpdateStatus(ctx, staleForJob); err != nil {
		t.Fatal(err)
	}
	if got, _ := chapters.GetByID(ctx, b.ChapterID); got.Status != domain.ChapterReady || got.Title != "Renamed too" {
		t.Errorf("job saved last: got %s %q, want ready %q", got.Status, got.Title, "Renamed too")
	}
}

// Migration 00002 deletes in-progress attempts (and their answers) and drops the
// status column, keeping submitted attempts as they were.
func TestMigrationDropsInProgressAttempts(t *testing.T) {
	ctx := context.Background()
	pool := freshDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}

	const (
		user, topic, chapter, set, question = "0190f5e2-0000-7000-8000-000000000001", "0190f5e2-0000-7000-8000-000000000002",
			"0190f5e2-0000-7000-8000-000000000003", "0190f5e2-0000-7000-8000-000000000004", "0190f5e2-0000-7000-8000-000000000005"
		submitted, inProgress = "0190f5e2-0000-7000-8000-00000000000a", "0190f5e2-0000-7000-8000-00000000000b"
	)
	seed := []string{
		`INSERT INTO users (id, email, name, created_at) VALUES ('` + user + `', 'ada@example.com', 'Ada', now())`,
		`INSERT INTO topics (id, owner_id, name, created_at) VALUES ('` + topic + `', '` + user + `', 'Bio', now())`,
		`INSERT INTO chapters (id, topic_id, owner_id, title, status, created_at) VALUES ('` + chapter + `', '` + topic + `', '` + user + `', 'Cells', 'ready', now())`,
		`INSERT INTO question_sets (id, chapter_id, created_at) VALUES ('` + set + `', '` + chapter + `', now())`,
		`INSERT INTO questions (id, question_set_id, position, type, prompt, options, correct, explanation)
			VALUES ('` + question + `', '` + set + `', 0, 'true_false', 'Cells?', '["True","False"]', '{0}', 'Yes.')`,
		`INSERT INTO chapter_attempts (id, owner_id, chapter_id, requested_count, question_ids, status, total_score, max_score, started_at, submitted_at)
			VALUES ('` + submitted + `', '` + user + `', '` + chapter + `', 5, '{` + question + `}', 'submitted', 0.5, 5, now(), now()),
			       ('` + inProgress + `', '` + user + `', '` + chapter + `', 5, '{` + question + `}', 'in_progress', 0, 5, now(), NULL)`,
		`INSERT INTO chapter_attempt_answers (attempt_id, question_id, position, selected, points, correct)
			VALUES ('` + submitted + `', '` + question + `', 0, '{0}', 0.5, false), ('` + inProgress + `', '` + question + `', 0, '{1}', 0, false)`,
	}
	for _, q := range seed {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("seed %.40s: %v", q, err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	var ids []string
	if err := pool.QueryRow(ctx, `SELECT array_agg(id::text) FROM chapter_attempts`).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{submitted}) {
		t.Errorf("attempts after migration: %v, want only the submitted one", ids)
	}
	var answers int
	var score float64
	if err := pool.QueryRow(ctx, `SELECT count(*), sum(ans.points)::float8 FROM chapter_attempt_answers ans`).Scan(&answers, &score); err != nil {
		t.Fatal(err)
	}
	if answers != 1 || score != 0.5 {
		t.Errorf("answers after migration: %d (points %v), want the submitted attempt's one", answers, score)
	}
	var hasStatus bool
	err = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chapter_attempts' AND column_name = 'status')`).Scan(&hasStatus)
	if err != nil || hasStatus {
		t.Errorf("status column still there: %v %v", hasStatus, err)
	}
}

// freshDB creates an empty database next to the shared test database, so a test can
// run migrations from the start.
func freshDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	admin := openDB(t)
	name := fmt.Sprintf("quizette_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(admin.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return pool
}
