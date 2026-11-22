package domain

import (
	"fmt"
	"time"
)

// JobStatus is the ProcessingJob lifecycle state.
type JobStatus string

// Job statuses. Transitions: queued -> running -> succeeded | failed; queued -> failed.
const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// Generation strategies (how the Document is fed to the LLM).
const (
	StrategyPDFNative   = "pdf-native"
	StrategyTextExtract = "text-extract"
)

// GenerationMetrics records cost and latency of generating a question set.
type GenerationMetrics struct {
	Strategy          string
	Model             string
	InputTokens       int
	OutputTokens      int
	ThinkingTokens    int
	TotalTokens       int
	ExtractMS         int64
	LLMMS             int64
	TotalMS           int64
	ExtractedChars    int
	ValidationRetries int
}

// Add accumulates metrics across LLM calls (e.g. a validation retry).
func (m GenerationMetrics) Add(o GenerationMetrics) GenerationMetrics {
	m.Strategy, m.Model = o.Strategy, o.Model
	m.InputTokens += o.InputTokens
	m.OutputTokens += o.OutputTokens
	m.ThinkingTokens += o.ThinkingTokens
	m.TotalTokens += o.TotalTokens
	m.ExtractMS += o.ExtractMS
	m.LLMMS += o.LLMMS
	m.TotalMS += o.TotalMS
	if o.ExtractedChars > 0 {
		m.ExtractedChars = o.ExtractedChars
	}
	return m
}

// ProcessingJob is one attempt to turn a Document into a QuestionSet
// (aggregate root). Mutate Status only through Start, Succeed and Fail.
type ProcessingJob struct {
	ID         ID
	ChapterID  ID
	OwnerID    ID
	Spec       QuestionSpec
	Document   DocumentMeta
	Status     JobStatus
	Attempts   int
	Error      string
	Metrics    GenerationMetrics
	QueuedAt   time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// NewProcessingJob creates a queued job.
func NewProcessingJob(id, chapterID, ownerID ID, spec QuestionSpec, doc DocumentMeta, now time.Time) ProcessingJob {
	return ProcessingJob{ID: id, ChapterID: chapterID, OwnerID: ownerID, Spec: spec, Document: doc, Status: JobQueued, QueuedAt: now}
}

// Start moves a queued job to running.
func (j *ProcessingJob) Start(now time.Time) error {
	if j.Status != JobQueued {
		return j.badTransition(JobRunning)
	}
	j.Status = JobRunning
	j.Attempts++
	j.StartedAt = &now
	return nil
}

// Succeed completes a running job.
func (j *ProcessingJob) Succeed(now time.Time, m GenerationMetrics) error {
	if j.Status != JobRunning {
		return j.badTransition(JobSucceeded)
	}
	j.Status = JobSucceeded
	j.Metrics = m
	j.FinishedAt = &now
	return nil
}

// Fail ends a queued or running job with a user-safe reason.
func (j *ProcessingJob) Fail(now time.Time, reason string, m GenerationMetrics) error {
	if j.IsTerminal() {
		return j.badTransition(JobFailed)
	}
	j.Status = JobFailed
	j.Error = reason
	j.Metrics = m
	j.FinishedAt = &now
	return nil
}

// IsTerminal reports whether the job has finished.
func (j ProcessingJob) IsTerminal() bool {
	return j.Status == JobSucceeded || j.Status == JobFailed
}

func (j ProcessingJob) badTransition(to JobStatus) error {
	return fmt.Errorf("%w: job %s -> %s", ErrInvalidTransition, j.Status, to)
}
