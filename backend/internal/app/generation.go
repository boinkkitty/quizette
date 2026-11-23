package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

// GenerationConfig tunes the generation service.
type GenerationConfig struct {
	MaxRetries int           // validation retries (default 1)
	JobTimeout time.Duration // per-job deadline
}

// GenerationService handles uploads and background processing.
type GenerationService struct {
	Deps
	Generator QuestionGenerator
	Queue     JobQueue
	Config    GenerationConfig
}

// UploadCommand is a request to create a chapter from a PDF.
type UploadCommand struct {
	OwnerID      domain.ID
	TopicID      domain.ID // existing topic, or
	NewTopicName string    // name of a topic to find-or-create
	Title        string    // defaults to the filename
	Count        int
	Filename     string
	Data         []byte
}

// UploadResult identifies what was created.
type UploadResult struct {
	TopicID   domain.ID
	ChapterID domain.ID
	JobID     domain.ID
}

// UploadDocument validates the upload, records a pending chapter and a queued
// job, and enqueues processing. The document is never persisted.
func (s *GenerationService) UploadDocument(ctx context.Context, cmd UploadCommand) (UploadResult, error) {
	doc, err := domain.NewDocument(cmd.Filename, cmd.Data)
	if err != nil {
		return UploadResult{}, err
	}
	spec, err := domain.NewQuestionSpec(cmd.Count, nil)
	if err != nil {
		return UploadResult{}, err
	}
	if cmd.TopicID == "" && strings.TrimSpace(cmd.NewTopicName) == "" {
		return UploadResult{}, domain.Invalid("choose a topic or enter a new topic name")
	}
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		title = strings.TrimSuffix(doc.Filename(), filepath.Ext(doc.Filename()))
	}

	now := s.Clock.Now()
	var res UploadResult
	err = s.UoW.Do(ctx, func(r Repos) error {
		topic, err := resolveTopic(ctx, r, s.IDs, cmd, now)
		if err != nil {
			return err
		}
		chapter, err := domain.NewChapter(s.IDs.NewID(), topic.ID, cmd.OwnerID, title, now)
		if err != nil {
			return err
		}
		if err := r.Chapters.Create(ctx, chapter); err != nil {
			return err
		}
		job := domain.NewProcessingJob(s.IDs.NewID(), chapter.ID, cmd.OwnerID, spec, doc.Meta(), now)
		if err := r.Jobs.Create(ctx, job); err != nil {
			return err
		}
		res = UploadResult{TopicID: topic.ID, ChapterID: chapter.ID, JobID: job.ID}
		return nil
	})
	if err != nil {
		return UploadResult{}, err
	}

	log := logctx.From(ctx, s.Logger).With(slog.String("job_id", res.JobID.String()), slog.String("chapter_id", res.ChapterID.String()))
	if err := s.Queue.Enqueue(ctx, res.JobID, doc); err != nil {
		log.ErrorContext(ctx, "job.enqueue_failed", slog.Any("error", err))
		s.finishFailed(ctx, res.JobID, "the server is busy; please try again shortly", domain.GenerationMetrics{})
		return UploadResult{}, fmt.Errorf("enqueue job: %w", err)
	}
	log.InfoContext(ctx, "job.queued", slog.Any("document", doc), slog.Int("count", spec.Count))
	return res, nil
}

func resolveTopic(ctx context.Context, r Repos, ids IDGenerator, cmd UploadCommand, now time.Time) (domain.Topic, error) {
	if cmd.TopicID != "" {
		return r.Topics.Get(ctx, cmd.OwnerID, cmd.TopicID)
	}
	t, err := domain.NewTopic(ids.NewID(), cmd.OwnerID, cmd.NewTopicName, now)
	if err != nil {
		return domain.Topic{}, err
	}
	existing, err := r.Topics.GetByName(ctx, cmd.OwnerID, t.Name)
	switch {
	case err == nil:
		return existing, nil
	case !errors.Is(err, domain.ErrNotFound):
		return domain.Topic{}, err
	}
	return t, r.Topics.Create(ctx, t)
}

// saveTimeout bounds saving a job's outcome once generation has finished or timed out.
const saveTimeout = 30 * time.Second

// ProcessJob runs generation for a queued job. Called by the JobQueue worker.
func (s *GenerationService) ProcessJob(ctx context.Context, jobID domain.ID, doc domain.Document) {
	log := logctx.From(ctx, s.Logger).With(slog.String("job_id", jobID.String()))
	ctx = logctx.With(ctx, log)
	if s.Config.JobTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Config.JobTimeout)
		defer cancel()
	}

	job, err := s.Repos.Jobs.GetByID(ctx, jobID)
	if err != nil {
		log.ErrorContext(ctx, "job.load_failed", slog.Any("error", err))
		return
	}
	log = log.With(slog.String("chapter_id", job.ChapterID.String()))
	ctx = logctx.With(ctx, log)

	started := s.Clock.Now()
	if err := job.Start(started); err != nil {
		log.WarnContext(ctx, "job.start_skipped", slog.Any("error", err))
		return
	}
	if err := s.Repos.Jobs.Update(ctx, job); err != nil {
		log.ErrorContext(ctx, "job.update_failed", slog.Any("error", err))
		return
	}
	log.InfoContext(ctx, "job.started", slog.Int("attempt", job.Attempts))

	set, metrics, err := s.generateSet(ctx, job, doc)
	metrics.TotalMS = s.Clock.Now().Sub(started).Milliseconds()

	// The outcome must be saved even when generation ran into the job deadline, so it
	// gets its own deadline instead of the (possibly expired) job context.
	ctx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancelSave()
	if err != nil {
		reason := userFacingReason(err)
		log.WarnContext(ctx, "job.failed", slog.String("reason", reason), slog.Any("error", err), slog.Int64("total_ms", metrics.TotalMS))
		s.finishFailed(ctx, job.ID, reason, metrics)
		return
	}

	err = s.UoW.Do(ctx, func(r Repos) error {
		if err := r.QuestionSets.Create(ctx, set); err != nil {
			return err
		}
		j, err := r.Jobs.GetByID(ctx, job.ID)
		if err != nil {
			return err
		}
		if err := j.Succeed(s.Clock.Now(), metrics); err != nil {
			return err
		}
		if err := r.Jobs.Update(ctx, j); err != nil {
			return err
		}
		ch, err := r.Chapters.GetByID(ctx, job.ChapterID)
		if err != nil {
			return err
		}
		if err := ch.MarkReady(); err != nil {
			return err
		}
		return r.Chapters.UpdateStatus(ctx, ch)
	})
	if err != nil {
		log.ErrorContext(ctx, "job.save_failed", slog.Any("error", err))
		s.finishFailed(ctx, job.ID, "could not save the generated questions", metrics)
		return
	}
	log.InfoContext(ctx, "job.succeeded",
		slog.Int("questions", set.Size()),
		slog.Int("total_tokens", metrics.TotalTokens),
		slog.Int64("total_ms", metrics.TotalMS),
		slog.Int("validation_retries", metrics.ValidationRetries))
}

func (s *GenerationService) generateSet(ctx context.Context, job domain.ProcessingJob, doc domain.Document) (domain.QuestionSet, domain.GenerationMetrics, error) {
	log := logctx.From(ctx, s.Logger)
	maxRetries := s.Config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	var (
		metrics  domain.GenerationMetrics
		feedback []string
		lastErr  error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			metrics.ValidationRetries++
		}
		res, err := s.Generator.Generate(ctx, GenerateInput{Document: doc, Spec: job.Spec, Feedback: feedback})
		metrics = metrics.Add(res.Metrics)
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			// The model answered, but not in a usable shape (e.g. JSON that doesn't match
			// the schema). Retry with the problem as feedback, like invalid drafts below.
			log.WarnContext(ctx, "job.output_invalid", slog.Any("violations", truncate(ve.Problems, 10)), slog.Int("retry", attempt+1))
			feedback, lastErr = truncate(ve.Problems, 20), err
			continue
		}
		if err != nil {
			return domain.QuestionSet{}, metrics, err
		}
		log.InfoContext(ctx, "job.llm_completed",
			slog.String("strategy", res.Metrics.Strategy),
			slog.String("model", res.Metrics.Model),
			slog.Int("input_tokens", res.Metrics.InputTokens),
			slog.Int("output_tokens", res.Metrics.OutputTokens),
			slog.Int("thinking_tokens", res.Metrics.ThinkingTokens),
			slog.Int64("llm_ms", res.Metrics.LLMMS),
			slog.Int64("extract_ms", res.Metrics.ExtractMS),
			slog.Int("extracted_chars", res.Metrics.ExtractedChars),
			slog.Int("drafts", len(res.Drafts)))

		set, discarded, err := domain.NewQuestionSet(s.IDs.NewID(), job.ChapterID, job.Spec, res.Drafts, s.IDs.NewID, s.Clock.Now())
		if err == nil {
			if len(discarded) > 0 {
				log.InfoContext(ctx, "job.drafts_discarded", slog.Int("discarded", len(discarded)))
			}
			return set, metrics, nil
		}
		if !errors.As(err, &ve) {
			return domain.QuestionSet{}, metrics, err
		}
		log.WarnContext(ctx, "job.validation_failed", slog.Int("problems", len(ve.Problems)), slog.Any("violations", truncate(ve.Problems, 10)), slog.Int("retry", attempt+1))
		feedback, lastErr = truncate(ve.Problems, 20), err
	}
	return domain.QuestionSet{}, metrics, lastErr
}

func truncate(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *GenerationService) finishFailed(ctx context.Context, jobID domain.ID, reason string, metrics domain.GenerationMetrics) {
	err := s.UoW.Do(ctx, func(r Repos) error {
		j, err := r.Jobs.GetByID(ctx, jobID)
		if err != nil {
			return err
		}
		if err := j.Fail(s.Clock.Now(), reason, metrics); err != nil {
			return err
		}
		if err := r.Jobs.Update(ctx, j); err != nil {
			return err
		}
		ch, err := r.Chapters.GetByID(ctx, j.ChapterID)
		if err != nil {
			return err
		}
		if ch.Status != domain.ChapterPending {
			return nil
		}
		if err := ch.MarkFailed(); err != nil {
			return err
		}
		return r.Chapters.UpdateStatus(ctx, ch)
	})
	if err != nil {
		logctx.From(ctx, s.Logger).ErrorContext(ctx, "job.mark_failed_failed", slog.String("job_id", jobID.String()), slog.Any("error", err))
	}
}

func userFacingReason(err error) string {
	var ve *domain.ValidationError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "question generation timed out; please try again"
	case errors.Is(err, ErrUnreadableDocument):
		return "we couldn't read any text from this PDF"
	case errors.As(err, &ve):
		return "the model did not produce enough valid questions; please try again"
	default:
		return "question generation failed; please try again"
	}
}

// RecoverInterrupted fails jobs left queued/running by a previous process.
// Their Documents were only in memory, so they cannot be resumed.
func (s *GenerationService) RecoverInterrupted(ctx context.Context) (int, error) {
	jobs, err := s.Repos.Jobs.ListUnfinished(ctx)
	if err != nil {
		return 0, err
	}
	for _, j := range jobs {
		s.finishFailed(ctx, j.ID, "the server restarted while processing; please re-upload the PDF", j.Metrics)
	}
	if len(jobs) > 0 {
		logctx.From(ctx, s.Logger).WarnContext(ctx, "job.recovered_interrupted", slog.Int("count", len(jobs)))
	}
	return len(jobs), nil
}

// GetJob returns a job owned by the user.
func (s *GenerationService) GetJob(ctx context.Context, ownerID, jobID domain.ID) (domain.ProcessingJob, error) {
	return s.Repos.Jobs.Get(ctx, ownerID, jobID)
}
