// Package app holds Quizette's use cases. It orchestrates the domain through
// ports (interfaces declared here, implemented by adapters). It must not
// import adapters, database drivers, HTTP routers or LLM SDKs.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// ---- Generation ports (the LLM swap point) ----

// GenerateInput is what a QuestionGenerator needs.
type GenerateInput struct {
	Document domain.Document
	Spec     domain.QuestionSpec
	// Feedback lists validation problems from a previous attempt, for retries.
	Feedback []string
}

// GenerationResult is a generator's output plus cost/latency metrics.
type GenerationResult struct {
	Drafts  []domain.QuestionDraft
	Metrics domain.GenerationMetrics
}

// QuestionGenerator turns a Document into question drafts (driven port).
// Implementations constrain output with QuestionSetSchema.
type QuestionGenerator interface {
	Generate(ctx context.Context, in GenerateInput) (GenerationResult, error)
}

// ErrLLMUnavailable marks a transient generator failure, such as a rate limit,
// an overloaded model or a timeout, where another model or provider may
// succeed. QuestionGenerator implementations wrap such errors with it.
var ErrLLMUnavailable = errors.New("question generator temporarily unavailable")

// ErrUnreadableDocument means no usable content could be read from a document.
var ErrUnreadableDocument = errors.New("document has no readable text")

// TextExtractor extracts plain text from a Document, for text-only LLM adapters.
type TextExtractor interface {
	Extract(ctx context.Context, doc domain.Document) (string, error)
}

// JobQueue runs ProcessingJobs in the background. The Document is held in
// memory only and dropped once the job finishes.
type JobQueue interface {
	Enqueue(ctx context.Context, jobID domain.ID, doc domain.Document) error
}

// ---- Repositories ----

// TopicSummary is a topic with its chapter count and the performance of its
// topic-scoped attempts. LastScorePct and BestScorePct are nil without attempts.
type TopicSummary struct {
	domain.Topic
	ChapterCount int
	AttemptCount int
	LastScorePct *float64
	BestScorePct *float64
}

// ChapterSummary is a chapter with list-view aggregates. AttemptCount and
// BestScorePct count chapter-scoped attempts only (ADR 0010).
type ChapterSummary struct {
	domain.Chapter
	TopicName     string
	QuestionCount int
	AttemptCount  int
	BestScorePct  *float64
}

// ChapterFilter narrows a chapter listing. Zero values mean "any".
type ChapterFilter struct {
	TopicID domain.ID
	Status  domain.ChapterStatus
}

// TopicRepository persists Topics. Create/Update return domain.ErrConflict on a duplicate name.
type TopicRepository interface {
	Create(ctx context.Context, t domain.Topic) error
	Get(ctx context.Context, ownerID, id domain.ID) (domain.Topic, error)
	GetByName(ctx context.Context, ownerID domain.ID, name string) (domain.Topic, error)
	List(ctx context.Context, ownerID domain.ID) ([]TopicSummary, error)
	Update(ctx context.Context, t domain.Topic) error
	Delete(ctx context.Context, ownerID, id domain.ID) error
}

// ChapterRepository persists Chapters.
type ChapterRepository interface {
	Create(ctx context.Context, c domain.Chapter) error
	Get(ctx context.Context, ownerID, id domain.ID) (domain.Chapter, error)
	GetByID(ctx context.Context, id domain.ID) (domain.Chapter, error)
	ListByTopic(ctx context.Context, ownerID, topicID domain.ID) ([]ChapterSummary, error)
	// ListByOwner lists the user's chapters matching filter, ordered by topic name then
	// creation time.
	ListByOwner(ctx context.Context, ownerID domain.ID, filter ChapterFilter) ([]ChapterSummary, error)
	// UpdateTitle saves c.Title only, and UpdateStatus saves c.Status only. Keeping them
	// apart means a rename and a generation job finishing at the same time can't
	// overwrite each other's change with a stale copy.
	UpdateTitle(ctx context.Context, c domain.Chapter) error
	UpdateStatus(ctx context.Context, c domain.Chapter) error
	Delete(ctx context.Context, ownerID, id domain.ID) error
}

// ProcessingJobRepository persists ProcessingJobs.
type ProcessingJobRepository interface {
	Create(ctx context.Context, j domain.ProcessingJob) error
	Get(ctx context.Context, ownerID, id domain.ID) (domain.ProcessingJob, error)
	GetByID(ctx context.Context, id domain.ID) (domain.ProcessingJob, error)
	LatestForChapter(ctx context.Context, ownerID, chapterID domain.ID) (domain.ProcessingJob, error)
	Update(ctx context.Context, j domain.ProcessingJob) error
	ListUnfinished(ctx context.Context) ([]domain.ProcessingJob, error)
}

// QuestionSetRepository persists QuestionSets (create-only; sets are immutable).
type QuestionSetRepository interface {
	Create(ctx context.Context, s domain.QuestionSet) error
	GetByChapter(ctx context.Context, chapterID domain.ID) (domain.QuestionSet, error)
}

// AttemptSummary is an Attempt with the names needed by an owner-wide history list.
// ChapterTitle is empty for a topic-scoped attempt. Answers are not loaded.
type AttemptSummary struct {
	domain.Attempt
	ChapterTitle string
	TopicID      domain.ID
	TopicName    string
}

// AttemptRepository persists Attempts.
type AttemptRepository interface {
	// Create saves a submitted attempt and its answers together.
	Create(ctx context.Context, a domain.Attempt) error
	Get(ctx context.Context, ownerID, id domain.ID) (domain.Attempt, error)
	// ListByChapter returns the chapter's chapter-scoped attempts, newest first.
	ListByChapter(ctx context.Context, ownerID, chapterID domain.ID) ([]domain.Attempt, error)
	// ListByOwner returns at most limit of the user's attempts across all scopes, newest first.
	ListByOwner(ctx context.Context, ownerID domain.ID, limit int) ([]AttemptSummary, error)
}

// Session is a login session; only the SHA-256 of its token is stored.
type Session struct {
	TokenHash string
	UserID    domain.ID
	ExpiresAt time.Time
	CreatedAt time.Time
}

// UserRepository persists users and OAuth links. Create returns ErrConflict on duplicate email.
type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	GetByID(ctx context.Context, id domain.ID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByOAuth(ctx context.Context, provider, providerUserID string) (domain.User, error)
	LinkOAuth(ctx context.Context, userID domain.ID, provider, providerUserID string) error
}

// SessionRepository persists sessions.
type SessionRepository interface {
	Create(ctx context.Context, s Session) error
	Get(ctx context.Context, tokenHash string) (Session, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

// Repos bundles repositories that share one transaction.
type Repos struct {
	Topics       TopicRepository
	Chapters     ChapterRepository
	Jobs         ProcessingJobRepository
	QuestionSets QuestionSetRepository
	Attempts     AttemptRepository
	Users        UserRepository
	Sessions     SessionRepository
}

// UnitOfWork runs fn atomically with transaction-scoped repositories.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(r Repos) error) error
}

// ---- Infrastructure ports ----

// Clock returns the current time.
type Clock interface{ Now() time.Time }

// IDGenerator creates new entity IDs.
type IDGenerator interface{ NewID() domain.ID }

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// OAuthProvider performs an OAuth2 authorization-code flow with PKCE.
type OAuthProvider interface {
	Name() string
	AuthCodeURL(state, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (domain.OAuthIdentity, error)
}
