package app

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// DashboardService builds the Dashboard from the user's chapters and attempts.
type DashboardService struct{ Deps }

// RecentWindow is how far back "Quizzes this week" counts, from now.
const RecentWindow = 7 * 24 * time.Hour

// DashboardStats are the Dashboard's stat tiles. ChaptersQuizzed counts ready
// chapters with a chapter-scoped attempt (ADR 0010), out of ReadyChapters.
type DashboardStats struct {
	AttemptStats
	ChaptersQuizzed int
	ReadyChapters   int
}

// DashboardTopic is one By topic row: of the topic's ready chapters, how many have a
// chapter-scoped attempt, and the mean of their last scores (nil when none has).
type DashboardTopic struct {
	domain.Topic
	ReadyChapters       int
	ChaptersQuizzed     int
	AverageLastScorePct *float64
}

// ActivityLimit caps how many items the Dashboard's Activity feed shows.
const ActivityLimit = 30

// ActivityItem is one item of the Activity feed: exactly one of Attempt, a submitted
// attempt, and Upload, a chapter that is not deleted, is set. At is when the attempt
// was submitted or the chapter uploaded.
type ActivityItem struct {
	At      time.Time
	Attempt *AttemptSummary
	Upload  *ChapterSummary
}

// Dashboard is everything the Dashboard shows. ChapterCount, GeneratingCount and
// FailedCount count the user's chapters that are not deleted.
type Dashboard struct {
	ChapterCount    int
	GeneratingCount int
	FailedCount     int
	Stats           DashboardStats
	// Topics are the topics that are not deleted, by name.
	Topics []DashboardTopic
	// Activity holds the newest ActivityLimit uploads and attempts, newest first.
	// Attempts on deleted chapters and topics stay, as in History.
	Activity []ActivityItem
}

// Get builds the user's Dashboard.
func (s *DashboardService) Get(ctx context.Context, ownerID domain.ID) (Dashboard, error) {
	topics, err := s.Repos.Topics.List(ctx, ownerID)
	if err != nil {
		return Dashboard{}, err
	}
	chapters, err := s.Repos.Chapters.ListByOwner(ctx, ownerID, ChapterFilter{})
	if err != nil {
		return Dashboard{}, err
	}
	attempts, err := s.Repos.Attempts.ListByOwner(ctx, ownerID, ActivityLimit)
	if err != nil {
		return Dashboard{}, err
	}
	stats, err := s.Repos.Attempts.Stats(ctx, ownerID, s.Clock.Now().Add(-RecentWindow))
	if err != nil {
		return Dashboard{}, err
	}
	d := Dashboard{ChapterCount: len(chapters), Stats: DashboardStats{AttemptStats: stats}, Topics: make([]DashboardTopic, len(topics))}
	rows := make(map[domain.ID]*DashboardTopic, len(topics))
	sums := make(map[domain.ID]float64, len(topics))
	for i, t := range topics {
		d.Topics[i] = DashboardTopic{Topic: t.Topic}
		rows[t.ID] = &d.Topics[i]
	}
	for _, c := range chapters {
		switch c.Status {
		case domain.ChapterPending:
			d.GeneratingCount++
		case domain.ChapterFailed:
			d.FailedCount++
		case domain.ChapterReady:
			d.Stats.ReadyChapters++
			quizzed := c.LastScorePct != nil
			if quizzed {
				d.Stats.ChaptersQuizzed++
			}
			// The topic is missing only if it was deleted between the two reads.
			if row := rows[c.TopicID]; row != nil {
				row.ReadyChapters++
				if quizzed {
					row.ChaptersQuizzed++
					sums[c.TopicID] += *c.LastScorePct
				}
			}
		}
	}
	for i := range d.Topics {
		if row := &d.Topics[i]; row.ChaptersQuizzed > 0 {
			avg := sums[row.ID] / float64(row.ChaptersQuizzed)
			row.AverageLastScorePct = &avg
		}
	}
	d.Activity = activity(attempts, chapters)
	return d, nil
}

// activity merges attempts and chapter uploads into the newest ActivityLimit items,
// newest first.
func activity(attempts []AttemptSummary, chapters []ChapterSummary) []ActivityItem {
	items := make([]ActivityItem, 0, len(attempts)+len(chapters))
	for i := range attempts {
		items = append(items, ActivityItem{At: attempts[i].SubmittedAt, Attempt: &attempts[i]})
	}
	for i := range chapters {
		items = append(items, ActivityItem{At: chapters[i].CreatedAt, Upload: &chapters[i]})
	}
	slices.SortStableFunc(items, func(a, b ActivityItem) int { return cmp.Compare(b.At.UnixNano(), a.At.UnixNano()) })
	return items[:min(len(items), ActivityLimit)]
}
