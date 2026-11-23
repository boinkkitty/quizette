// Package inprocess implements app.JobQueue with a bounded goroutine pool.
// Documents live only in memory; jobs lost on restart are failed at boot by
// app.GenerationService.RecoverInterrupted.
package inprocess

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// ErrQueueFull is returned when the buffer is full.
var ErrQueueFull = errors.New("job queue is full")

// ErrClosed is returned after Shutdown.
var ErrClosed = errors.New("job queue is closed")

// Handler processes one job.
type Handler func(ctx context.Context, jobID domain.ID, doc domain.Document)

type item struct {
	ctx   context.Context //nolint:containedctx // carries request-scoped log values to the worker
	jobID domain.ID
	doc   domain.Document
}

// Queue runs jobs on a fixed number of workers.
type Queue struct {
	handler Handler
	logger  *slog.Logger
	items   chan item
	base    context.Context //nolint:containedctx // cancelled on shutdown
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
}

// New starts workers goroutines with a buffer of size buffer.
func New(workers, buffer int, handler Handler, logger *slog.Logger) *Queue {
	base, cancel := context.WithCancel(context.Background())
	q := &Queue{handler: handler, logger: logger, items: make(chan item, buffer), base: base, cancel: cancel}
	for range max(workers, 1) {
		q.wg.Go(q.work)
	}
	return q
}

func (q *Queue) work() {
	for it := range q.items {
		// Keep the request's values (logger, IDs) but not its cancellation;
		// cancel instead when the queue shuts down.
		ctx, cancel := context.WithCancel(context.WithoutCancel(it.ctx))
		stop := context.AfterFunc(q.base, cancel)
		func() {
			defer func() {
				if r := recover(); r != nil {
					q.logger.ErrorContext(ctx, "job.panic", slog.String("job_id", it.jobID.String()), slog.Any("panic", r))
				}
			}()
			q.handler(ctx, it.jobID, it.doc)
		}()
		stop()
		cancel()
	}
}

// Enqueue implements app.JobQueue without blocking.
func (q *Queue) Enqueue(ctx context.Context, jobID domain.ID, doc domain.Document) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return ErrClosed
	}
	select {
	case q.items <- item{ctx: ctx, jobID: jobID, doc: doc}:
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown stops accepting jobs and waits for running ones until ctx expires,
// then cancels them.
func (q *Queue) Shutdown(ctx context.Context) {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.items)
	}
	q.mu.Unlock()
	done := make(chan struct{})
	go func() { q.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		q.cancel()
		<-done
	}
	q.cancel()
}
