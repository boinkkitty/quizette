//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
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

	quiz, err := prac.DrawQuiz(ctx, u.ID, domain.ChapterScope(res.ChapterID), 5)
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
	v, err := prac.SubmitQuiz(ctx, u.ID, domain.ChapterScope(res.ChapterID), drawn, responses)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prac.GetAttempt(ctx, u.ID, v.Attempt.ID)
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
	v2, err := prac.SubmitQuiz(ctx, u.ID, domain.ChapterScope(res.ChapterID), drawn, nil)
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

	// A perfect topic quiz counts towards the topic only, not the chapter it drew from.
	topicScope := domain.TopicScope(res.TopicID)
	tq, err := prac.DrawQuiz(ctx, u.ID, topicScope, 10)
	if err != nil {
		t.Fatal(err)
	}
	tdrawn := make([]domain.ID, len(tq.Questions))
	perfect := map[domain.ID][]int{}
	for i, q := range tq.Questions {
		tdrawn[i], perfect[q.ID] = q.ID, q.Correct
	}
	time.Sleep(time.Millisecond)
	tv, err := prac.SubmitQuiz(ctx, u.ID, topicScope, tdrawn, perfect)
	if err != nil {
		t.Fatal(err)
	}
	gotTopic, err := prac.GetAttempt(ctx, u.ID, tv.Attempt.ID)
	if err != nil || gotTopic.Attempt.Scope != topicScope || gotTopic.ScopeName != "Biology" || len(gotTopic.Questions) != 10 ||
		gotTopic.Questions[0].ChapterID != res.ChapterID || gotTopic.Attempt.TotalScore != 10 {
		t.Fatalf("topic attempt: %v %+v", err, gotTopic)
	}
	topics, err := lib.ListTopics(ctx, u.ID)
	if err != nil || len(topics) != 1 || topics[0].AttemptCount != 1 || topics[0].LastScorePct == nil || *topics[0].LastScorePct != 100 ||
		topics[0].BestScorePct == nil || *topics[0].BestScorePct != 100 {
		t.Fatalf("topic performance: %v %+v", err, topics)
	}
	chapters, err := lib.ListChapters(ctx, u.ID, app.ChapterFilter{})
	if err != nil || len(chapters) != 1 || chapters[0].AttemptCount != 2 || *chapters[0].BestScorePct != 80 {
		t.Fatalf("chapter performance after a topic quiz: %v %+v", err, chapters)
	}
	if byChapter, err := prac.ListChapterAttempts(ctx, u.ID, res.ChapterID); err != nil || len(byChapter) != 2 {
		t.Fatalf("chapter history after a topic quiz: %v %d attempts", err, len(byChapter))
	}
	hist, err = prac.ListAttempts(ctx, u.ID)
	if err != nil || len(hist) != 3 || hist[0].ID != tv.Attempt.ID || hist[0].Scope != topicScope || hist[0].ChapterTitle != "" ||
		hist[0].TopicID != res.TopicID || hist[0].TopicName != "Biology" {
		t.Fatalf("history with a topic attempt: %v %+v", err, hist)
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

	// Deleting a topic that attempts refer to keeps it and its chapters as deleted
	// (ADR 0010): hidden, but the attempts keep their questions.
	if err := lib.DeleteTopic(ctx, u.ID, res.TopicID); err != nil {
		t.Fatal(err)
	}
	if topics, err := lib.ListTopics(ctx, u.ID); err != nil || len(topics) != 0 {
		t.Fatalf("topics after delete: %v %+v", err, topics)
	}
	if _, err := lib.GetChapter(ctx, u.ID, res.ChapterID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("chapter of deleted topic: %v", err)
	}
	kept, err := prac.GetAttempt(ctx, u.ID, tv.Attempt.ID)
	if err != nil || !kept.ScopeDeleted || kept.ScopeName != "Biology" || len(kept.Questions) != 10 || !kept.Questions[0].ChapterDeleted {
		t.Fatalf("topic attempt after delete: %v %+v", err, kept)
	}
	if hist, err := prac.ListAttempts(ctx, u.ID); err != nil || len(hist) != 3 || !hist[0].ScopeDeleted || !hist[1].ScopeDeleted {
		t.Fatalf("history after delete: %v %+v", err, hist)
	}
	if _, err := prac.SubmitQuiz(ctx, u.ID, topicScope, tdrawn, nil); !errors.Is(err, domain.ErrDeletedContent) {
		t.Fatalf("submit naming deleted questions: %v", err)
	}

	// The name is free again, and a topic no attempt refers to is removed with its questions.
	fresh, err := lib.CreateTopic(ctx, u.ID, "biology")
	if err != nil {
		t.Fatalf("reuse deleted topic's name: %v", err)
	}
	if _, err := lib.CreateTopic(ctx, u.ID, "Biology"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate of a live topic's name: %v", err)
	}
	var before, after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM questions").Scan(&before); err != nil {
		t.Fatal(err)
	}
	queue.Handler = gen.ProcessJob
	if _, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, TopicID: fresh.ID, Count: 5, Filename: "c.pdf", Data: []byte("%PDF-1.4 c")}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM questions").Scan(&after); err != nil || after != before+5 {
		t.Fatalf("questions after uploading to the new topic: %d, want %d (%v)", after, before+5, err)
	}
	if err := lib.DeleteTopic(ctx, u.ID, fresh.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM questions").Scan(&after); err != nil || after != before {
		t.Fatalf("questions after removing an unreferenced topic: %d, want %d (%v)", after, before, err)
	}
	if _, err := store.Repos().Topics.GetIncludingDeleted(ctx, u.ID, fresh.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unreferenced topic was kept: %v", err)
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

	if _, err := provider.UpTo(ctx, 2); err != nil {
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

// Migration 00003 turns chapter attempts into chapter-scoped attempts (ADR 0010),
// keeping their answers and scores.
func TestMigrationScopesExistingAttempts(t *testing.T) {
	ctx := context.Background()
	pool := freshDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 2); err != nil {
		t.Fatal(err)
	}

	const (
		user, topic, chapter, set, question = "0190f5e2-0000-7000-8000-000000000001", "0190f5e2-0000-7000-8000-000000000002",
			"0190f5e2-0000-7000-8000-000000000003", "0190f5e2-0000-7000-8000-000000000004", "0190f5e2-0000-7000-8000-000000000005"
		attempt = "0190f5e2-0000-7000-8000-00000000000a"
	)
	seed := []string{
		`INSERT INTO users (id, email, name, created_at) VALUES ('` + user + `', 'ada@example.com', 'Ada', now())`,
		`INSERT INTO topics (id, owner_id, name, created_at) VALUES ('` + topic + `', '` + user + `', 'Bio', now())`,
		`INSERT INTO chapters (id, topic_id, owner_id, title, status, created_at) VALUES ('` + chapter + `', '` + topic + `', '` + user + `', 'Cells', 'ready', now())`,
		`INSERT INTO question_sets (id, chapter_id, created_at) VALUES ('` + set + `', '` + chapter + `', now())`,
		`INSERT INTO questions (id, question_set_id, position, type, prompt, options, correct, explanation)
			VALUES ('` + question + `', '` + set + `', 0, 'true_false', 'Cells?', '["True","False"]', '{0}', 'Yes.')`,
		`INSERT INTO chapter_attempts (id, owner_id, chapter_id, requested_count, question_ids, total_score, max_score, started_at, submitted_at)
			VALUES ('` + attempt + `', '` + user + `', '` + chapter + `', 5, '{` + question + `}', 0.5, 5, now(), now())`,
		`INSERT INTO chapter_attempt_answers (attempt_id, question_id, position, selected, points, correct)
			VALUES ('` + attempt + `', '` + question + `', 0, '{0}', 0.5, false)`,
	}
	for _, q := range seed {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("seed %.40s: %v", q, err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	var scope string
	var chapterID, topicID *string
	var total float64
	var answers int
	err = pool.QueryRow(ctx, `SELECT a.scope, a.chapter_id::text, a.topic_id::text, a.total_score::float8,
		(SELECT count(*) FROM attempt_answers ans WHERE ans.attempt_id = a.id)::int
		FROM attempts a WHERE a.id = $1`, attempt).Scan(&scope, &chapterID, &topicID, &total, &answers)
	if err != nil {
		t.Fatal(err)
	}
	if scope != "chapter" || chapterID == nil || *chapterID != chapter || topicID != nil || total != 0.5 || answers != 1 {
		t.Errorf("attempt after migration: scope %q chapter %v topic %v score %v answers %d; want a chapter-scoped attempt scoring 0.5 with its answer",
			scope, chapterID, topicID, total, answers)
	}

	// Exactly one of topic_id and chapter_id is set, matching the scope.
	for name, cols := range map[string]string{
		"topic scope naming a chapter": `'topic', NULL, '` + chapter + `'`,
		"topic scope naming both":      `'topic', '` + topic + `', '` + chapter + `'`,
		"chapter scope naming a topic": `'chapter', '` + topic + `', NULL`,
	} {
		_, err := pool.Exec(ctx, `INSERT INTO attempts (id, owner_id, scope, topic_id, chapter_id, requested_count, question_ids, total_score, max_score, started_at, submitted_at)
			VALUES (gen_random_uuid(), '`+user+`', `+cols+`, 5, '{}', 0, 5, now(), now())`)
		if err == nil {
			t.Errorf("%s: insert succeeded, want a check violation", name)
		}
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

func TestHistoryCountsTheChaptersAQuizDrewFrom(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	_, gen, lib, prac, u := newServices(t, pool, true)

	cells, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, NewTopicName: "Biology", Count: 10, Filename: "cells.pdf", Data: []byte("%PDF-1.4 cells")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, TopicID: cells.TopicID, Count: 10, Filename: "genes.pdf", Data: []byte("%PDF-1.4 genes")}); err != nil {
		t.Fatal(err)
	}
	submit := func(scope domain.AttemptScope) {
		t.Helper()
		q, err := prac.DrawQuiz(ctx, u.ID, scope, 10)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]domain.ID, len(q.Questions))
		for i, dq := range q.Questions {
			ids[i] = dq.ID
		}
		if _, err := prac.SubmitQuiz(ctx, u.ID, scope, ids, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // Attempts are listed by started_at, so space them apart.
	}
	submit(domain.TopicScope(cells.TopicID)) // The draw splits 10 questions 5 and 5 over the two chapters.
	submit(domain.ChapterScope(cells.ChapterID))
	// A deleted chapter still counts: the attempt drew from it.
	if err := lib.DeleteChapter(ctx, u.ID, cells.ChapterID); err != nil {
		t.Fatal(err)
	}

	hist, err := prac.ListAttempts(ctx, u.ID)
	if err != nil || len(hist) != 2 {
		t.Fatalf("history: %v %+v", err, hist)
	}
	if hist[0].Scope.Type != domain.ScopeChapter || hist[0].ChapterCount != 1 {
		t.Errorf("chapter quiz: %+v, want ChapterCount 1", hist[0])
	}
	if hist[1].Scope.Type != domain.ScopeTopic || hist[1].ChapterCount != 2 {
		t.Errorf("topic quiz: %+v, want ChapterCount 2", hist[1])
	}
}

func TestDashboardFigures(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	store, gen, _, prac, u := newServices(t, pool, true)
	dash := &app.DashboardService{Deps: app.Deps{Repos: store.Repos(), UoW: store, Clock: system.Clock{}, IDs: system.UUIDv7{}, Logger: slog.New(slog.DiscardHandler)}}

	cells, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, NewTopicName: "Biology", Count: 10, Filename: "cells.pdf", Data: []byte("%PDF-1.4 cells")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.UploadDocument(ctx, app.UploadCommand{OwnerID: u.ID, TopicID: cells.TopicID, Count: 10, Filename: "genes.pdf", Data: []byte("%PDF-1.4 genes")}); err != nil {
		t.Fatal(err)
	}
	// submit takes a quiz of count questions, answered as the stub grades correct if
	// correct is set and left unanswered otherwise.
	submit := func(scope domain.AttemptScope, count int, correct bool) {
		t.Helper()
		q, err := prac.DrawQuiz(ctx, u.ID, scope, count)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]domain.ID, len(q.Questions))
		answers := map[domain.ID][]int{}
		for i, dq := range q.Questions {
			ids[i] = dq.ID
			if correct {
				answers[dq.ID] = map[domain.QuestionType][]int{domain.MCQ: {1}, domain.TrueFalse: {0}, domain.MRQ: {0, 2}}[dq.Type]
			}
		}
		if _, err := prac.SubmitQuiz(ctx, u.ID, scope, ids, answers); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // Space the attempts apart so "last" is well defined.
	}
	submit(domain.ChapterScope(cells.ChapterID), 5, true)  // 100%
	submit(domain.ChapterScope(cells.ChapterID), 5, false) // 0%: Cells' last score, though not its best
	submit(domain.TopicScope(cells.TopicID), 10, true)     // 100%, sets no chapter's last score

	d, err := dash.Get(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := d.Stats
	if s.Count != 3 || s.AverageScorePct == nil || math.Abs(*s.AverageScorePct-200.0/3) > 1e-9 || s.Questions != 20 || s.RecentCount != 3 {
		t.Errorf("stats = %+v (average %v), want 3 attempts averaging 66.67 over 20 questions, all recent", s, s.AverageScorePct)
	}
	if s.ChaptersQuizzed != 1 || s.ReadyChapters != 2 {
		t.Errorf("chapters quizzed = %d of %d, want 1 of 2", s.ChaptersQuizzed, s.ReadyChapters)
	}
	if len(d.Topics) != 1 || d.Topics[0].AverageLastScorePct == nil || *d.Topics[0].AverageLastScorePct != 0 {
		t.Errorf("topics = %+v, want Biology averaging Cells' last score, 0", d.Topics)
	}
	if len(d.Activity) != 5 || d.Activity[0].Attempt == nil || d.Activity[0].Attempt.Scope.Type != domain.ScopeTopic {
		t.Errorf("activity = %+v, want 3 attempts and 2 uploads, the topic quiz first", d.Activity)
	}
}
