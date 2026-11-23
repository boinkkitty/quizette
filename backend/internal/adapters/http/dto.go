package http

import (
	"math"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type userDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

func toUserDTO(u domain.User) userDTO {
	return userDTO{ID: u.ID.String(), Email: u.Email, Name: u.Name, AvatarURL: u.AvatarURL}
}

type topicDTO struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ChapterCount int       `json:"chapterCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type chapterSummaryDTO struct {
	ID            string    `json:"id"`
	TopicID       string    `json:"topicId"`
	TopicName     string    `json:"topicName"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	QuestionCount int       `json:"questionCount"`
	AttemptCount  int       `json:"attemptCount"`
	BestScorePct  *float64  `json:"bestScorePct"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toChapterSummary(c app.ChapterSummary) chapterSummaryDTO {
	return chapterSummaryDTO{
		ID: c.ID.String(), TopicID: c.TopicID.String(), TopicName: c.TopicName, Title: c.Title, Status: string(c.Status),
		QuestionCount: c.QuestionCount, AttemptCount: c.AttemptCount, BestScorePct: c.BestScorePct, CreatedAt: c.CreatedAt,
	}
}

type jobDTO struct {
	ID             string     `json:"id"`
	ChapterID      string     `json:"chapterId"`
	Status         string     `json:"status"`
	Error          string     `json:"error,omitempty"`
	RequestedCount int        `json:"requestedCount"`
	DocumentName   string     `json:"documentName"`
	DocumentSize   int64      `json:"documentSize"`
	Attempts       int        `json:"attempts"`
	Strategy       string     `json:"strategy,omitempty"`
	Model          string     `json:"model,omitempty"`
	TotalTokens    int        `json:"totalTokens"`
	TotalMS        int64      `json:"totalMs"`
	QueuedAt       time.Time  `json:"queuedAt"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
}

func toJobDTO(j domain.ProcessingJob) jobDTO {
	return jobDTO{
		ID: j.ID.String(), ChapterID: j.ChapterID.String(), Status: string(j.Status), Error: j.Error,
		RequestedCount: j.Spec.Count, DocumentName: j.Document.Filename, DocumentSize: j.Document.Size, Attempts: j.Attempts,
		Strategy: j.Metrics.Strategy, Model: j.Metrics.Model, TotalTokens: j.Metrics.TotalTokens, TotalMS: j.Metrics.TotalMS,
		QueuedAt: j.QueuedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

type chapterDTO struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	TopicID       string    `json:"topicId"`
	TopicName     string    `json:"topicName"`
	QuestionCount int       `json:"questionCount"`
	LatestJob     *jobDTO   `json:"latestJob"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toChapterDTO(d app.ChapterDetail) chapterDTO {
	out := chapterDTO{
		ID: d.Chapter.ID.String(), Title: d.Chapter.Title, Status: string(d.Chapter.Status),
		TopicID: d.Topic.ID.String(), TopicName: d.Topic.Name, QuestionCount: d.QuestionCount, CreatedAt: d.Chapter.CreatedAt,
	}
	if d.LatestJob != nil {
		j := toJobDTO(*d.LatestJob)
		out.LatestJob = &j
	}
	return out
}

// questionDTO omits answers unless reveal is true.
type questionDTO struct {
	ID          string   `json:"id"`
	Position    int      `json:"position"`
	Type        string   `json:"type"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Correct     []int    `json:"correct,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	SourceHint  string   `json:"sourceHint,omitempty"`
}

func toQuestionDTO(q domain.Question, reveal bool) questionDTO {
	d := questionDTO{ID: q.ID.String(), Position: q.Position, Type: string(q.Type), Prompt: q.Prompt, Options: q.Options}
	if reveal {
		d.Correct, d.Explanation, d.SourceHint = q.Correct, q.Explanation, q.SourceHint
	}
	return d
}

type answerDTO struct {
	QuestionID string  `json:"questionId"`
	Selected   []int   `json:"selected"`
	Points     float64 `json:"points"`
	Correct    bool    `json:"correct"`
}

type attemptSummaryDTO struct {
	ID          string     `json:"id"`
	ChapterID   string     `json:"chapterId"`
	Status      string     `json:"status"`
	Count       int        `json:"count"`
	TotalScore  float64    `json:"totalScore"`
	MaxScore    int        `json:"maxScore"`
	ScorePct    float64    `json:"scorePct"`
	StartedAt   time.Time  `json:"startedAt"`
	SubmittedAt *time.Time `json:"submittedAt"`
}

func toAttemptSummary(a domain.ChapterAttempt) attemptSummaryDTO {
	pct := 0.0
	if a.MaxScore > 0 {
		pct = math.Round(a.TotalScore/float64(a.MaxScore)*10000) / 100
	}
	return attemptSummaryDTO{
		ID: a.ID.String(), ChapterID: a.ChapterID.String(), Status: string(a.Status), Count: a.RequestedCount,
		TotalScore: a.TotalScore, MaxScore: a.MaxScore, ScorePct: pct, StartedAt: a.StartedAt, SubmittedAt: a.SubmittedAt,
	}
}

// attemptHistoryItemDTO is one row of the owner-wide attempt history.
type attemptHistoryItemDTO struct {
	attemptSummaryDTO
	ChapterTitle string `json:"chapterTitle"`
	TopicID      string `json:"topicId"`
	TopicName    string `json:"topicName"`
}

type attemptDTO struct {
	attemptSummaryDTO
	ChapterTitle string        `json:"chapterTitle"`
	Questions    []questionDTO `json:"questions"`
	Answers      []answerDTO   `json:"answers"`
}

// toAttemptDTO reveals correct answers only after submission.
func toAttemptDTO(v app.AttemptView) attemptDTO {
	reveal := v.Attempt.Status == domain.AttemptSubmitted
	out := attemptDTO{attemptSummaryDTO: toAttemptSummary(v.Attempt), ChapterTitle: v.ChapterTitle, Questions: make([]questionDTO, len(v.Questions)), Answers: []answerDTO{}}
	for i, q := range v.Questions {
		out.Questions[i] = toQuestionDTO(q, reveal)
	}
	for _, a := range v.Attempt.Answers {
		out.Answers = append(out.Answers, answerDTO{QuestionID: a.QuestionID.String(), Selected: a.Selected, Points: a.Score.Points, Correct: a.Score.Correct})
	}
	return out
}
