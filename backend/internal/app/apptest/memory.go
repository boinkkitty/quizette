// Package apptest provides in-memory implementations of the app ports for
// tests of use cases and driving adapters.
package apptest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Store is a thread-safe in-memory database implementing every repository.
type Store struct {
	mu       sync.Mutex
	topics   map[domain.ID]domain.Topic
	chapters map[domain.ID]domain.Chapter
	jobs     map[domain.ID]domain.ProcessingJob
	sets     map[domain.ID]domain.QuestionSet // by chapter ID
	attempts map[domain.ID]domain.ChapterAttempt
	users    map[domain.ID]domain.User
	oauth    map[string]domain.ID
	sessions map[string]app.Session
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		topics: map[domain.ID]domain.Topic{}, chapters: map[domain.ID]domain.Chapter{},
		jobs: map[domain.ID]domain.ProcessingJob{}, sets: map[domain.ID]domain.QuestionSet{},
		attempts: map[domain.ID]domain.ChapterAttempt{}, users: map[domain.ID]domain.User{},
		oauth: map[string]domain.ID{}, sessions: map[string]app.Session{},
	}
}

// Repos returns repositories backed by the store.
func (s *Store) Repos() app.Repos {
	return app.Repos{
		Topics: topicRepo{s}, Chapters: chapterRepo{s}, Jobs: jobRepo{s},
		QuestionSets: setRepo{s}, Attempts: attemptRepo{s}, Users: userRepo{s}, Sessions: sessionRepo{s},
	}
}

// Do implements app.UnitOfWork (no rollback; adequate for tests).
// Do runs fn with the store's repositories. Like a real database transaction, it refuses
// to start once ctx is done.
func (s *Store) Do(ctx context.Context, fn func(app.Repos) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	return fn(s.Repos())
}

// Job returns a job by ID (test helper).
func (s *Store) Job(id domain.ID) domain.ProcessingJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id]
}

// DeleteUsers removes every user (test helper for orphaned sessions).
func (s *Store) DeleteUsers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.users)
}

// Chapter returns a chapter by ID (test helper).
func (s *Store) Chapter(id domain.ID) domain.Chapter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chapters[id]
}

func notFound(what string) error { return fmt.Errorf("%s: %w", what, domain.ErrNotFound) }

type topicRepo struct{ s *Store }

func (r topicRepo) Create(_ context.Context, t domain.Topic) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, o := range r.s.topics {
		if o.OwnerID == t.OwnerID && strings.EqualFold(o.Name, t.Name) {
			return fmt.Errorf("topic name taken: %w", domain.ErrConflict)
		}
	}
	r.s.topics[t.ID] = t
	return nil
}

func (r topicRepo) Get(_ context.Context, owner, id domain.ID) (domain.Topic, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.topics[id]
	if !ok || t.OwnerID != owner {
		return domain.Topic{}, notFound("topic")
	}
	return t, nil
}

func (r topicRepo) GetByName(_ context.Context, owner domain.ID, name string) (domain.Topic, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.topics {
		if t.OwnerID == owner && strings.EqualFold(t.Name, name) {
			return t, nil
		}
	}
	return domain.Topic{}, notFound("topic")
}

func (r topicRepo) List(_ context.Context, owner domain.ID) ([]app.TopicSummary, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []app.TopicSummary
	for _, t := range r.s.topics {
		if t.OwnerID != owner {
			continue
		}
		n := 0
		for _, c := range r.s.chapters {
			if c.TopicID == t.ID {
				n++
			}
		}
		out = append(out, app.TopicSummary{Topic: t, ChapterCount: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r topicRepo) Update(ctx context.Context, t domain.Topic) error {
	if _, err := r.Get(ctx, t.OwnerID, t.ID); err != nil {
		return err
	}
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.topics[t.ID] = t
	return nil
}

func (r topicRepo) Delete(ctx context.Context, owner, id domain.ID) error {
	if _, err := r.Get(ctx, owner, id); err != nil {
		return err
	}
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	delete(r.s.topics, id)
	for cid, c := range r.s.chapters {
		if c.TopicID == id {
			delete(r.s.chapters, cid)
		}
	}
	return nil
}

type chapterRepo struct{ s *Store }

func (r chapterRepo) Create(_ context.Context, c domain.Chapter) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.chapters[c.ID] = c
	return nil
}

func (r chapterRepo) Get(ctx context.Context, owner, id domain.ID) (domain.Chapter, error) {
	c, err := r.GetByID(ctx, id)
	if err != nil || c.OwnerID != owner {
		return domain.Chapter{}, notFound("chapter")
	}
	return c, nil
}

func (r chapterRepo) GetByID(_ context.Context, id domain.ID) (domain.Chapter, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.chapters[id]
	if !ok {
		return domain.Chapter{}, notFound("chapter")
	}
	return c, nil
}

func (r chapterRepo) ListByTopic(_ context.Context, owner, topicID domain.ID) ([]app.ChapterSummary, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := r.s.chapterSummaries(func(c domain.Chapter) bool { return c.OwnerID == owner && c.TopicID == topicID })
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r chapterRepo) ListByOwner(_ context.Context, owner domain.ID, filter app.ChapterFilter) ([]app.ChapterSummary, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := r.s.chapterSummaries(func(c domain.Chapter) bool {
		return c.OwnerID == owner &&
			(filter.TopicID == "" || c.TopicID == filter.TopicID) &&
			(filter.Status == "" || c.Status == filter.Status)
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].TopicName != out[j].TopicName {
			return out[i].TopicName < out[j].TopicName
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// chapterSummaries builds summaries for chapters matching keep. The caller holds s.mu.
func (s *Store) chapterSummaries(keep func(domain.Chapter) bool) []app.ChapterSummary {
	var out []app.ChapterSummary
	for _, c := range s.chapters {
		if !keep(c) {
			continue
		}
		cs := app.ChapterSummary{Chapter: c, TopicName: s.topics[c.TopicID].Name, QuestionCount: s.sets[c.ID].Size()}
		for _, a := range s.attempts {
			if a.ChapterID == c.ID {
				cs.AttemptCount++
			}
		}
		out = append(out, cs)
	}
	return out
}

func (r chapterRepo) UpdateTitle(_ context.Context, c domain.Chapter) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	stored, ok := r.s.chapters[c.ID]
	if !ok || stored.OwnerID != c.OwnerID {
		return notFound("chapter")
	}
	stored.Title = c.Title
	r.s.chapters[c.ID] = stored
	return nil
}

func (r chapterRepo) UpdateStatus(_ context.Context, c domain.Chapter) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	stored, ok := r.s.chapters[c.ID]
	if !ok {
		return notFound("chapter")
	}
	stored.Status = c.Status
	r.s.chapters[c.ID] = stored
	return nil
}

func (r chapterRepo) Delete(ctx context.Context, owner, id domain.ID) error {
	if _, err := r.Get(ctx, owner, id); err != nil {
		return err
	}
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	delete(r.s.chapters, id)
	delete(r.s.sets, id)
	return nil
}

type jobRepo struct{ s *Store }

func (r jobRepo) Create(_ context.Context, j domain.ProcessingJob) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.jobs[j.ID] = j
	return nil
}

func (r jobRepo) Get(ctx context.Context, owner, id domain.ID) (domain.ProcessingJob, error) {
	j, err := r.GetByID(ctx, id)
	if err != nil || j.OwnerID != owner {
		return domain.ProcessingJob{}, notFound("job")
	}
	return j, nil
}

func (r jobRepo) GetByID(_ context.Context, id domain.ID) (domain.ProcessingJob, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	j, ok := r.s.jobs[id]
	if !ok {
		return domain.ProcessingJob{}, notFound("job")
	}
	return j, nil
}

func (r jobRepo) LatestForChapter(_ context.Context, owner, chapterID domain.ID) (domain.ProcessingJob, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var best *domain.ProcessingJob
	for _, j := range r.s.jobs {
		if j.OwnerID == owner && j.ChapterID == chapterID && (best == nil || j.QueuedAt.After(best.QueuedAt)) {
			best = &j
		}
	}
	if best == nil {
		return domain.ProcessingJob{}, notFound("job")
	}
	return *best, nil
}

func (r jobRepo) Update(_ context.Context, j domain.ProcessingJob) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.jobs[j.ID] = j
	return nil
}

func (r jobRepo) ListUnfinished(_ context.Context) ([]domain.ProcessingJob, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.ProcessingJob
	for _, j := range r.s.jobs {
		if !j.IsTerminal() {
			out = append(out, j)
		}
	}
	return out, nil
}

type setRepo struct{ s *Store }

func (r setRepo) Create(_ context.Context, set domain.QuestionSet) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.sets[set.ChapterID]; ok {
		return fmt.Errorf("set exists: %w", domain.ErrConflict)
	}
	r.s.sets[set.ChapterID] = set
	return nil
}

func (r setRepo) GetByChapter(_ context.Context, chapterID domain.ID) (domain.QuestionSet, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	set, ok := r.s.sets[chapterID]
	if !ok {
		return domain.QuestionSet{}, notFound("question set")
	}
	return set, nil
}

type attemptRepo struct{ s *Store }

func (r attemptRepo) Create(_ context.Context, a domain.ChapterAttempt) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.attempts[a.ID] = a
	return nil
}

func (r attemptRepo) Get(_ context.Context, owner, id domain.ID) (domain.ChapterAttempt, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	a, ok := r.s.attempts[id]
	if !ok || a.OwnerID != owner {
		return domain.ChapterAttempt{}, notFound("attempt")
	}
	return a, nil
}

func (r attemptRepo) ListByChapter(_ context.Context, owner, chapterID domain.ID) ([]domain.ChapterAttempt, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.ChapterAttempt
	for _, a := range r.s.attempts {
		if a.OwnerID == owner && a.ChapterID == chapterID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

func (r attemptRepo) ListByOwner(_ context.Context, owner domain.ID, limit int) ([]app.AttemptSummary, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []app.AttemptSummary
	for _, a := range r.s.attempts {
		if a.OwnerID != owner {
			continue
		}
		ch := r.s.chapters[a.ChapterID]
		a.Answers = nil // the summary does not load answers, like the Postgres adapter
		out = append(out, app.AttemptSummary{ChapterAttempt: a, ChapterTitle: ch.Title, TopicID: ch.TopicID, TopicName: r.s.topics[ch.TopicID].Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type userRepo struct{ s *Store }

func (r userRepo) Create(_ context.Context, u domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, o := range r.s.users {
		if o.Email == u.Email {
			return fmt.Errorf("email taken: %w", domain.ErrConflict)
		}
	}
	r.s.users[u.ID] = u
	return nil
}

func (r userRepo) GetByID(_ context.Context, id domain.ID) (domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return domain.User{}, notFound("user")
	}
	return u, nil
}

func (r userRepo) GetByEmail(_ context.Context, email string) (domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.Email == email {
			return u, nil
		}
	}
	return domain.User{}, notFound("user")
}

func (r userRepo) GetByOAuth(ctx context.Context, provider, puid string) (domain.User, error) {
	r.s.mu.Lock()
	id, ok := r.s.oauth[provider+":"+puid]
	r.s.mu.Unlock()
	if !ok {
		return domain.User{}, notFound("user")
	}
	return r.GetByID(ctx, id)
}

func (r userRepo) LinkOAuth(_ context.Context, userID domain.ID, provider, puid string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.oauth[provider+":"+puid] = userID
	return nil
}

type sessionRepo struct{ s *Store }

func (r sessionRepo) Create(_ context.Context, sess app.Session) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.sessions[sess.TokenHash] = sess
	return nil
}

func (r sessionRepo) Get(_ context.Context, hash string) (app.Session, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	sess, ok := r.s.sessions[hash]
	if !ok {
		return app.Session{}, notFound("session")
	}
	return sess, nil
}

func (r sessionRepo) Delete(_ context.Context, hash string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	delete(r.s.sessions, hash)
	return nil
}

func (r sessionRepo) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var n int64
	for h, sess := range r.s.sessions {
		if sess.ExpiresAt.Before(now) {
			delete(r.s.sessions, h)
			n++
		}
	}
	return n, nil
}

// ---- infrastructure fakes ----

// Clock is a settable fake clock.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a clock fixed at t.
func NewClock(t time.Time) *Clock { return &Clock{now: t} }

// Now implements app.Clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// IDs generates sequential IDs ("id-1", "id-2", ...).
type IDs struct{ n atomic.Int64 }

// NewID implements app.IDGenerator.
func (g *IDs) NewID() domain.ID { return domain.ID(fmt.Sprintf("id-%d", g.n.Add(1))) }

// Hasher is a reversible fake password hasher.
type Hasher struct{}

// Hash implements app.PasswordHasher.
func (Hasher) Hash(p string) (string, error) { return "hashed:" + p, nil }

// Compare implements app.PasswordHasher.
func (Hasher) Compare(h, p string) bool { return h == "hashed:"+p }

// SyncQueue runs jobs synchronously through Handler.
type SyncQueue struct {
	Handler func(ctx context.Context, jobID domain.ID, doc domain.Document)
	Err     error
}

// Enqueue implements app.JobQueue.
func (q *SyncQueue) Enqueue(ctx context.Context, jobID domain.ID, doc domain.Document) error {
	if q.Err != nil {
		return q.Err
	}
	if q.Handler != nil {
		q.Handler(ctx, jobID, doc)
	}
	return nil
}

// Generator is a scripted QuestionGenerator.
type Generator struct {
	mu     sync.Mutex
	Calls  []app.GenerateInput
	Script []func(app.GenerateInput) (app.GenerationResult, error)
}

// Generate implements app.QuestionGenerator; it runs Script[i] on call i
// (the last entry repeats).
func (g *Generator) Generate(_ context.Context, in app.GenerateInput) (app.GenerationResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Calls = append(g.Calls, in)
	i := min(len(g.Calls)-1, len(g.Script)-1)
	return g.Script[i](in)
}

// OAuthProvider is a fake provider returning Identity.
type OAuthProvider struct {
	ProviderName string
	Identity     domain.OAuthIdentity
	Err          error
}

// Name implements app.OAuthProvider.
func (p OAuthProvider) Name() string { return p.ProviderName }

// AuthCodeURL implements app.OAuthProvider.
func (p OAuthProvider) AuthCodeURL(state, _ string) string {
	return "https://auth.example/" + p.ProviderName + "?state=" + state
}

// Exchange implements app.OAuthProvider.
func (p OAuthProvider) Exchange(context.Context, string, string) (domain.OAuthIdentity, error) {
	return p.Identity, p.Err
}

// ValidDrafts returns n valid drafts cycling MCQ, True/False and MRQ.
func ValidDrafts(n int) []domain.QuestionDraft {
	out := make([]domain.QuestionDraft, n)
	for i := range out {
		switch i % 3 {
		case 0:
			out[i] = domain.QuestionDraft{Type: domain.MCQ, Prompt: fmt.Sprintf("MCQ %d", i), Options: []string{"a", "b", "c", "d"}, Correct: []int{1}, Explanation: "b is right"}
		case 1:
			out[i] = domain.QuestionDraft{Type: domain.TrueFalse, Prompt: fmt.Sprintf("TF %d", i), Options: []string{"True", "False"}, Correct: []int{0}, Explanation: "it is true"}
		default:
			out[i] = domain.QuestionDraft{Type: domain.MRQ, Prompt: fmt.Sprintf("MRQ %d", i), Options: []string{"a", "b", "c", "d"}, Correct: []int{0, 2}, Explanation: "a and c"}
		}
	}
	return out
}

// Returns is a Generator script step producing drafts with fixed metrics.
func Returns(drafts []domain.QuestionDraft) func(app.GenerateInput) (app.GenerationResult, error) {
	return func(app.GenerateInput) (app.GenerationResult, error) {
		return app.GenerationResult{Drafts: drafts, Metrics: domain.GenerationMetrics{Strategy: "fake", Model: "fake", InputTokens: 100, OutputTokens: 50, TotalTokens: 150, LLMMS: 10}}, nil
	}
}

// Fails is a Generator script step returning err.
func Fails(err error) func(app.GenerateInput) (app.GenerationResult, error) {
	return func(app.GenerateInput) (app.GenerationResult, error) { return app.GenerationResult{}, err }
}
