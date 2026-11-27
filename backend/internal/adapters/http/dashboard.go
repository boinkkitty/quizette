package http

import (
	"net/http"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
)

type dashboardStatsDTO struct {
	AverageScorePct   *float64 `json:"averageScorePct"`
	AttemptCount      int      `json:"attemptCount"`
	QuizzesThisWeek   int      `json:"quizzesThisWeek"`
	QuestionsAnswered int      `json:"questionsAnswered"`
	ChaptersQuizzed   int      `json:"chaptersQuizzed"`
	ReadyChapters     int      `json:"readyChapters"`
}

type dashboardTopicDTO struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	ReadyChapters       int      `json:"readyChapters"`
	ChaptersQuizzed     int      `json:"chaptersQuizzed"`
	AverageLastScorePct *float64 `json:"averageLastScorePct"`
}

// activityItemDTO is one item of the Activity feed. kind is "attempt" or "upload",
// and id is the attempt's or the chapter's. An attempt carries scope, scorePct and
// count, and chapterId and chapterTitle for a chapter quiz; an upload carries status
// and questionCount.
type activityItemDTO struct {
	Kind          string    `json:"kind"`
	ID            string    `json:"id"`
	At            time.Time `json:"at"`
	ChapterID     string    `json:"chapterId,omitempty"`
	ChapterTitle  string    `json:"chapterTitle"`
	TopicID       string    `json:"topicId"`
	TopicName     string    `json:"topicName"`
	Scope         string    `json:"scope,omitempty"`
	ScorePct      *float64  `json:"scorePct,omitempty"`
	Count         int       `json:"count,omitempty"`
	Status        string    `json:"status,omitempty"`
	QuestionCount *int      `json:"questionCount,omitempty"`
}

func toActivityItemDTO(it app.ActivityItem) activityItemDTO {
	if c := it.Upload; c != nil {
		return activityItemDTO{
			Kind: "upload", ID: c.ID.String(), At: it.At, ChapterTitle: c.Title,
			TopicID: c.TopicID.String(), TopicName: c.TopicName, Status: string(c.Status), QuestionCount: &c.QuestionCount,
		}
	}
	a := it.Attempt
	sum := toAttemptSummary(a.Attempt)
	return activityItemDTO{
		Kind: "attempt", ID: sum.ID, At: it.At, ChapterID: sum.ChapterID, ChapterTitle: a.ChapterTitle,
		TopicID: a.TopicID.String(), TopicName: a.TopicName, Scope: sum.Scope, ScorePct: &sum.ScorePct, Count: sum.Count,
	}
}

type dashboardDTO struct {
	ChapterCount    int                 `json:"chapterCount"`
	GeneratingCount int                 `json:"generatingCount"`
	FailedCount     int                 `json:"failedCount"`
	Stats           dashboardStatsDTO   `json:"stats"`
	Topics          []dashboardTopicDTO `json:"topics"`
	Activity        []activityItemDTO   `json:"activity"`
}

func toDashboardDTO(d app.Dashboard) dashboardDTO {
	s := d.Stats
	topics := make([]dashboardTopicDTO, len(d.Topics))
	for i, t := range d.Topics {
		topics[i] = dashboardTopicDTO{
			ID: t.ID.String(), Name: t.Name, ReadyChapters: t.ReadyChapters,
			ChaptersQuizzed: t.ChaptersQuizzed, AverageLastScorePct: t.AverageLastScorePct,
		}
	}
	activity := make([]activityItemDTO, len(d.Activity))
	for i, it := range d.Activity {
		activity[i] = toActivityItemDTO(it)
	}
	return dashboardDTO{
		Topics: topics, Activity: activity,
		ChapterCount: d.ChapterCount, GeneratingCount: d.GeneratingCount, FailedCount: d.FailedCount,
		Stats: dashboardStatsDTO{
			AverageScorePct: s.AverageScorePct, AttemptCount: s.Count, QuizzesThisWeek: s.RecentCount,
			QuestionsAnswered: s.Questions, ChaptersQuizzed: s.ChaptersQuizzed, ReadyChapters: s.ReadyChapters,
		},
	}
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	d, err := s.cfg.Dashboard.Get(r.Context(), mustUser(r).ID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDashboardDTO(d))
}
