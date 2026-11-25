package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/app/apptest"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

var pdf = []byte("%PDF-1.7 fake content")

type env struct {
	store *apptest.Store
	clock *apptest.Clock
	gen   *apptest.Generator
	queue *apptest.SyncQueue
	logs  *bytes.Buffer
	deps  app.Deps
	genS  *app.GenerationService
	lib   *app.LibraryService
	prac  *app.PracticeService
	ids   *app.IdentityService
}

func newEnv(t *testing.T, script ...func(app.GenerateInput) (app.GenerationResult, error)) *env {
	t.Helper()
	store := apptest.NewStore()
	logs := &bytes.Buffer{}
	e := &env{store: store, clock: apptest.NewClock(time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)), logs: logs}
	e.deps = app.Deps{
		Repos: store.Repos(), UoW: store, Clock: e.clock, IDs: &apptest.IDs{},
		Logger:  slog.New(slog.NewJSONHandler(logs, nil)),
		NewRand: func() *rand.Rand { return rand.New(rand.NewPCG(1, 1)) },
	}
	if len(script) == 0 {
		script = append(script, apptest.Returns(apptest.ValidDrafts(30)))
	}
	e.gen = &apptest.Generator{Script: script}
	e.queue = &apptest.SyncQueue{}
	e.genS = &app.GenerationService{Deps: e.deps, Generator: e.gen, Queue: e.queue, Config: app.GenerationConfig{MaxRetries: 1}}
	e.queue.Handler = e.genS.ProcessJob
	e.lib = &app.LibraryService{Deps: e.deps}
	e.prac = &app.PracticeService{Deps: e.deps}
	e.ids = &app.IdentityService{Deps: e.deps, Hasher: apptest.Hasher{}, Providers: map[string]app.OAuthProvider{}}
	return e
}

func (e *env) upload(t *testing.T, owner domain.ID, count int) app.UploadResult {
	t.Helper()
	res, err := e.genS.UploadDocument(context.Background(), app.UploadCommand{OwnerID: owner, NewTopicName: "Biology", Count: count, Filename: "cells.pdf", Data: pdf})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	return res
}

func TestUploadAndProcessSuccess(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	res := e.upload(t, "u1", 10)

	job := e.store.Job(res.JobID)
	if job.Status != domain.JobSucceeded || job.Metrics.TotalTokens != 150 || job.Document.Filename != "cells.pdf" {
		t.Fatalf("job = %+v", job)
	}
	ch := e.store.Chapter(res.ChapterID)
	if ch.Status != domain.ChapterReady || ch.Title != "cells" {
		t.Fatalf("chapter = %+v", ch)
	}
	set, err := e.lib.GetQuestionSet(context.Background(), "u1", res.ChapterID)
	if err != nil || set.Size() != 10 {
		t.Fatalf("set size %d err %v", set.Size(), err)
	}
	// Second upload with the same topic name reuses the topic.
	res2 := e.upload(t, "u1", 5)
	if res2.TopicID != res.TopicID {
		t.Errorf("topic not reused: %s vs %s", res2.TopicID, res.TopicID)
	}
	// Logs: lifecycle events present, no document content.
	logs := e.logs.String()
	for _, ev := range []string{"job.queued", "job.started", "job.llm_completed", "job.succeeded"} {
		if !strings.Contains(logs, ev) {
			t.Errorf("missing log event %s", ev)
		}
	}
	if strings.Contains(logs, "fake content") {
		t.Error("document content leaked into logs")
	}
}

func TestUploadValidation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	tests := []struct {
		name string
		cmd  app.UploadCommand
	}{
		{"not pdf", app.UploadCommand{OwnerID: "u", NewTopicName: "T", Count: 5, Data: []byte("hello")}},
		{"bad count", app.UploadCommand{OwnerID: "u", NewTopicName: "T", Count: 7, Data: pdf}},
		{"no topic", app.UploadCommand{OwnerID: "u", Count: 5, Data: pdf}},
		{"foreign topic", app.UploadCommand{OwnerID: "u", TopicID: "nope", Count: 5, Data: pdf}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.genS.UploadDocument(context.Background(), tt.cmd)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) && !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("want validation/not found error, got %v", err)
			}
		})
	}
}

func TestProcessRetriesOnInvalidOutputThenSucceeds(t *testing.T) {
	t.Parallel()
	bad := apptest.ValidDrafts(5)
	bad[0].Correct = []int{9}
	e := newEnv(t, apptest.Returns(bad), apptest.Returns(apptest.ValidDrafts(5)))
	res := e.upload(t, "u1", 5)

	job := e.store.Job(res.JobID)
	if job.Status != domain.JobSucceeded || job.Metrics.ValidationRetries != 1 || job.Metrics.TotalTokens != 300 {
		t.Fatalf("job = %+v", job)
	}
	if len(e.gen.Calls) != 2 || len(e.gen.Calls[1].Feedback) == 0 {
		t.Fatalf("retry feedback not passed: %+v", e.gen.Calls)
	}
}

func TestProcessFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		script func(app.GenerateInput) (app.GenerationResult, error)
		reason string
	}{
		{"llm error", apptest.Fails(errors.New("boom")), "question generation failed"},
		{"unreadable", apptest.Fails(app.ErrUnreadableDocument), "couldn't read any text"},
		{"timeout", apptest.Fails(context.DeadlineExceeded), "timed out"},
		{"always invalid", apptest.Returns(apptest.ValidDrafts(2)), "enough valid questions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, tt.script)
			res := e.upload(t, "u1", 5)
			job := e.store.Job(res.JobID)
			if job.Status != domain.JobFailed || !strings.Contains(job.Error, tt.reason) {
				t.Fatalf("job = %+v", job)
			}
			if ch := e.store.Chapter(res.ChapterID); ch.Status != domain.ChapterFailed {
				t.Fatalf("chapter status %s", ch.Status)
			}
		})
	}
}

func TestEnqueueFailureMarksFailed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.queue.Err = errors.New("full")
	_, err := e.genS.UploadDocument(context.Background(), app.UploadCommand{OwnerID: "u1", NewTopicName: "T", Count: 5, Data: pdf})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestRecoverInterrupted(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.queue.Handler = nil // leave jobs queued
	res := e.upload(t, "u1", 5)
	n, err := e.genS.RecoverInterrupted(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if j := e.store.Job(res.JobID); j.Status != domain.JobFailed || !strings.Contains(j.Error, "re-upload") {
		t.Fatalf("job = %+v", j)
	}
	if ch := e.store.Chapter(res.ChapterID); ch.Status != domain.ChapterFailed {
		t.Fatalf("chapter = %+v", ch)
	}
}

func TestIdentity(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()

	u, tok, err := e.ids.SignUp(ctx, "Ada@Example.com", "correct horse", "")
	if err != nil || u.Email != "ada@example.com" || u.Name != "ada" {
		t.Fatalf("signup: %v %+v", err, u)
	}
	if !tok.ExpiresAt.Equal(e.clock.Now().Add(app.SessionTTL)) {
		t.Errorf("session expiry = %v", tok.ExpiresAt)
	}
	if _, _, err := e.ids.SignUp(ctx, "ada@example.com", "another pass", ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("dup signup: %v", err)
	}
	got, err := e.ids.Authenticate(ctx, string(tok.Token))
	if err != nil || got.ID != u.ID {
		t.Fatalf("auth: %v", err)
	}
	if _, _, err := e.ids.LogIn(ctx, "ada@example.com", "wrong pass"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("bad password: %v", err)
	}
	if _, _, err := e.ids.LogIn(ctx, "nobody@example.com", "whatever1"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("unknown email: %v", err)
	}
	_, tok2, err := e.ids.LogIn(ctx, "ADA@example.com", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ids.LogOut(ctx, string(tok2.Token)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ids.Authenticate(ctx, string(tok2.Token)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("after logout: %v", err)
	}
	if _, err := e.ids.Authenticate(ctx, string(tok.Token)); err != nil {
		t.Fatalf("logout must only end its own session: %v", err)
	}
	e.clock.Advance(app.SessionTTL + time.Second)
	if _, err := e.ids.Authenticate(ctx, string(tok.Token)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired: %v", err)
	}
}

func TestAuthenticateRejectsInvalidTokens(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	for _, tok := range []string{"", "nope"} {
		if _, err := e.ids.Authenticate(ctx, tok); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%q: %v", tok, err)
		}
	}
	if err := e.ids.LogOut(ctx, ""); err != nil {
		t.Errorf("empty logout: %v", err)
	}

	// A session whose user no longer exists is unauthorized, not an error.
	_, tok, err := e.ids.SignUp(ctx, "gone@example.com", "correct horse", "")
	if err != nil {
		t.Fatal(err)
	}
	e.store.DeleteUsers()
	if _, err := e.ids.Authenticate(ctx, string(tok.Token)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("orphaned session: %v", err)
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	if _, _, err := e.ids.SignUp(ctx, "ada@example.com", "correct horse", ""); err != nil {
		t.Fatal(err)
	}
	if n, err := e.ids.PurgeExpiredSessions(ctx); err != nil || n != 0 {
		t.Fatalf("fresh session purged: %d %v", n, err)
	}
	e.clock.Advance(app.SessionTTL + time.Second)
	if n, err := e.ids.PurgeExpiredSessions(ctx); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
}

func TestOAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("links verified email to existing account", func(t *testing.T) {
		e := newEnv(t)
		u, _, _ := e.ids.SignUp(ctx, "ada@example.com", "correct horse", "Ada")
		e.ids.Providers["github"] = apptest.OAuthProvider{ProviderName: "github", Identity: domain.OAuthIdentity{Provider: "github", ProviderUserID: "42", Email: "ada@example.com", EmailVerified: true}}
		got, _, err := e.ids.CompleteOAuth(ctx, "github", "code", "verifier")
		if err != nil || got.ID != u.ID {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("unverified email does not take over account", func(t *testing.T) {
		e := newEnv(t)
		_, _, _ = e.ids.SignUp(ctx, "ada@example.com", "correct horse", "Ada")
		e.ids.Providers["github"] = apptest.OAuthProvider{ProviderName: "github", Identity: domain.OAuthIdentity{ProviderUserID: "42", Email: "ada@example.com"}}
		if _, _, err := e.ids.CompleteOAuth(ctx, "github", "code", "v"); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("want conflict, got %v", err)
		}
	})
	t.Run("creates new user then reuses link", func(t *testing.T) {
		e := newEnv(t)
		e.ids.Providers["google"] = apptest.OAuthProvider{ProviderName: "google", Identity: domain.OAuthIdentity{ProviderUserID: "g1", Email: "new@example.com", EmailVerified: true, Name: "New"}}
		u1, _, err := e.ids.CompleteOAuth(ctx, "google", "c", "v")
		if err != nil || u1.HasPassword() {
			t.Fatalf("%v %+v", err, u1)
		}
		u2, _, err := e.ids.CompleteOAuth(ctx, "google", "c", "v")
		if err != nil || u2.ID != u1.ID {
			t.Fatalf("second login: %v", err)
		}
		start, err := e.ids.StartOAuth("google")
		if err != nil || start.State == "" || !strings.Contains(start.URL, start.State) {
			t.Fatalf("start: %+v %v", start, err)
		}
	})
}

// blockingGenerator never answers: it waits until the job's deadline passes, like a
// hung LLM call.
type blockingGenerator struct{}

func (blockingGenerator) Generate(ctx context.Context, _ app.GenerateInput) (app.GenerationResult, error) {
	<-ctx.Done()
	return app.GenerationResult{}, fmt.Errorf("generate: %w", ctx.Err())
}

// Regression: a job that hit JobTimeout stayed "running" (and its chapter "pending")
// forever, because the failure was saved with the already-expired job context.
func TestProcessJobTimeoutMarksJobFailed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.genS.Generator = blockingGenerator{}
	e.genS.Config.JobTimeout = 20 * time.Millisecond
	res := e.upload(t, "u1", 5)

	job := e.store.Job(res.JobID)
	if job.Status != domain.JobFailed || !strings.Contains(job.Error, "timed out") {
		t.Fatalf("job = %s %q, want failed with a timeout reason", job.Status, job.Error)
	}
	if ch := e.store.Chapter(res.ChapterID); ch.Status != domain.ChapterFailed {
		t.Fatalf("chapter = %s, want failed", ch.Status)
	}
}

// Regression: when the generator itself reported malformed output (a validation error,
// e.g. JSON that doesn't match the schema), the job failed at once instead of retrying
// with that feedback.
func TestProcessJobRetriesMalformedGeneratorOutput(t *testing.T) {
	t.Parallel()
	e := newEnv(t,
		apptest.Fails(domain.Invalid("response was not valid JSON matching the schema")),
		apptest.Returns(apptest.ValidDrafts(5)),
	)
	res := e.upload(t, "u1", 5)

	job := e.store.Job(res.JobID)
	if job.Status != domain.JobSucceeded || job.Metrics.ValidationRetries != 1 {
		t.Fatalf("job = %s retries=%d, want succeeded after 1 retry", job.Status, job.Metrics.ValidationRetries)
	}
	if len(e.gen.Calls) != 2 || !slices.ContainsFunc(e.gen.Calls[1].Feedback, func(f string) bool { return strings.Contains(f, "not valid JSON") }) {
		t.Fatalf("retry feedback = %v, want the malformed-output problem", e.gen.Calls)
	}
}

// When every model or provider is busy, the user is told so instead of a generic failure.
func TestProcessJobUnavailableReason(t *testing.T) {
	t.Parallel()
	e := newEnv(t, apptest.Fails(fmt.Errorf("%w: 503 overloaded", app.ErrLLMUnavailable)))
	res := e.upload(t, "u1", 5)

	job := e.store.Job(res.JobID)
	if job.Status != domain.JobFailed || !strings.Contains(job.Error, "busy") {
		t.Fatalf("job = %s %q, want failed with a busy reason", job.Status, job.Error)
	}
}
