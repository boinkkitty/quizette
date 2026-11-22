package inprocess_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/queue/inprocess"
	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

func TestQueueRunsJobsAndDetachesRequestCancellation(t *testing.T) {
	t.Parallel()
	var ran atomic.Int32
	gotLogger := make(chan bool, 1)
	logger := slog.New(slog.DiscardHandler)
	q := inprocess.New(2, 4, func(ctx context.Context, _ domain.ID, _ domain.Document) {
		time.Sleep(20 * time.Millisecond)
		if ctx.Err() == nil {
			ran.Add(1)
		}
		select {
		case gotLogger <- logctx.From(ctx, nil) == logger:
		default:
		}
	}, logger)

	reqCtx, cancel := context.WithCancel(logctx.With(context.Background(), logger))
	if err := q.Enqueue(reqCtx, "j1", domain.Document{}); err != nil {
		t.Fatal(err)
	}
	cancel() // request finished; job must keep running
	q.Shutdown(context.Background())
	if ran.Load() != 1 {
		t.Fatalf("ran = %d", ran.Load())
	}
	if !<-gotLogger {
		t.Error("request logger not propagated")
	}
	if err := q.Enqueue(context.Background(), "j2", domain.Document{}); !errors.Is(err, inprocess.ErrClosed) {
		t.Errorf("after shutdown: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	q := inprocess.New(1, 1, func(context.Context, domain.ID, domain.Document) { <-block }, slog.New(slog.DiscardHandler))
	_ = q.Enqueue(context.Background(), "a", domain.Document{})
	time.Sleep(10 * time.Millisecond) // worker picks up "a"
	_ = q.Enqueue(context.Background(), "b", domain.Document{})
	if err := q.Enqueue(context.Background(), "c", domain.Document{}); !errors.Is(err, inprocess.ErrQueueFull) {
		t.Fatalf("want ErrQueueFull, got %v", err)
	}
	close(block)
	q.Shutdown(context.Background())
}

func TestShutdownCancelsAfterDeadline(t *testing.T) {
	t.Parallel()
	cancelled := make(chan bool, 1)
	q := inprocess.New(1, 1, func(ctx context.Context, _ domain.ID, _ domain.Document) {
		<-ctx.Done()
		cancelled <- true
	}, slog.New(slog.DiscardHandler))
	_ = q.Enqueue(context.Background(), "a", domain.Document{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	q.Shutdown(ctx)
	if !<-cancelled {
		t.Fatal("job not cancelled")
	}
}
