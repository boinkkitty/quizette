package http_test

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
)

// dashboard is the body of GET /api/dashboard.
type dashboard struct {
	ChapterCount    int `json:"chapterCount"`
	GeneratingCount int `json:"generatingCount"`
	FailedCount     int `json:"failedCount"`
	Stats           struct {
		AverageScorePct   *float64 `json:"averageScorePct"`
		AttemptCount      int      `json:"attemptCount"`
		QuizzesThisWeek   int      `json:"quizzesThisWeek"`
		QuestionsAnswered int      `json:"questionsAnswered"`
		ChaptersQuizzed   int      `json:"chaptersQuizzed"`
		ReadyChapters     int      `json:"readyChapters"`
	} `json:"stats"`
	Topics []struct {
		ID                  string   `json:"id"`
		Name                string   `json:"name"`
		ReadyChapters       int      `json:"readyChapters"`
		ChaptersQuizzed     int      `json:"chaptersQuizzed"`
		AverageLastScorePct *float64 `json:"averageLastScorePct"`
	} `json:"topics"`
	Activity []activityItem `json:"activity"`
}

// activityItem is one row of the Dashboard's Activity feed: a submitted attempt or
// a chapter upload.
type activityItem struct {
	Kind          string   `json:"kind"`
	ID            string   `json:"id"`
	At            string   `json:"at"`
	Scope         string   `json:"scope"`
	ChapterID     string   `json:"chapterId"`
	ChapterTitle  string   `json:"chapterTitle"`
	TopicID       string   `json:"topicId"`
	TopicName     string   `json:"topicName"`
	ScorePct      *float64 `json:"scorePct"`
	Count         int      `json:"count"`
	Status        string   `json:"status"`
	QuestionCount int      `json:"questionCount"`
}

// row describes an activity item in a few words, for comparing feeds.
func (a activityItem) row() string {
	if a.Kind == "upload" {
		return fmt.Sprintf("upload %s %s/%s %s %d", a.ID, a.TopicName, a.ChapterTitle, a.Status, a.QuestionCount)
	}
	return fmt.Sprintf("attempt %s %s %s/%s %v%% of %d", a.ID, a.Scope, a.TopicName, a.ChapterTitle, deref(a.ScorePct), a.Count)
}

func getDashboard(t *testing.T, c *client) dashboard {
	t.Helper()
	var d dashboard
	if code := c.do("GET", "/api/dashboard", nil, &d); code != http.StatusOK {
		t.Fatalf("dashboard: %d", code)
	}
	return d
}

// quiz draws count questions from the scope in body and submits them, answered
// correctly if correct is set and left unanswered otherwise. It returns the attempt ID.
func quiz(t *testing.T, c *client, body map[string]any, count int, correct bool) string {
	t.Helper()
	var q drawnQuiz
	draw := map[string]any{"count": count}
	for k, v := range body {
		draw[k] = v
	}
	if code := c.do("POST", "/api/quizzes/draw", draw, &q); code != http.StatusOK {
		t.Fatalf("draw %v: %d", draw, code)
	}
	answers := []map[string]any{}
	for _, dq := range q.Questions {
		if correct {
			sel := map[string][]int{"mcq": {1}, "true_false": {0}, "mrq": {0, 2}}[dq.Type]
			answers = append(answers, map[string]any{"questionId": dq.ID, "selected": sel})
		}
	}
	submit := map[string]any{"questionIds": questionIDs(q), "answers": answers}
	for k, v := range body {
		submit[k] = v
	}
	var a attempt
	if code := c.do("POST", "/api/attempts", submit, &a); code != http.StatusCreated {
		t.Fatalf("submit %v: %d", body, code)
	}
	return a.ID
}

func TestDashboardStats(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, _ := biologyTopic(t, h, c) // Cells and Genes ready, Broken failed

	// A chapter left generating: the queue holds the job instead of running it.
	run := h.queue.Handler
	h.queue.Handler = nil
	var up map[string]string
	if code := c.upload(map[string]string{"topicId": topicID, "count": "5", "title": "Pending"}, "pending.pdf", pdftest.Minimal("pending"), &up); code != http.StatusAccepted {
		t.Fatalf("upload pending: %d", code)
	}
	h.queue.Handler = run

	quiz(t, c, map[string]any{"chapterId": cellsID}, 5, false) // 0%, more than a week ago
	h.clock.Advance(8 * 24 * time.Hour)
	quiz(t, c, map[string]any{"chapterId": cellsID}, 5, true) // 100%, Cells' last score
	quiz(t, c, map[string]any{"topicId": topicID}, 10, true)  // 100%, quizzes no chapter on its own

	d := getDashboard(t, c)
	if d.ChapterCount != 4 || d.GeneratingCount != 1 || d.FailedCount != 1 {
		t.Errorf("chapters = %d, generating %d, failed %d; want 4, 1, 1", d.ChapterCount, d.GeneratingCount, d.FailedCount)
	}
	s := d.Stats
	if s.AverageScorePct == nil || math.Abs(*s.AverageScorePct-200.0/3) > 1e-9 {
		t.Errorf("average score = %v, want 66.67 (the mean of 0, 100 and 100)", deref(s.AverageScorePct))
	}
	if s.AttemptCount != 3 || s.QuizzesThisWeek != 2 || s.QuestionsAnswered != 20 {
		t.Errorf("attempts %d, this week %d, questions %d; want 3, 2, 20", s.AttemptCount, s.QuizzesThisWeek, s.QuestionsAnswered)
	}
	if s.ChaptersQuizzed != 1 || s.ReadyChapters != 2 {
		t.Errorf("chapters quizzed = %d of %d, want 1 of 2: a topic quiz quizzes no chapter", s.ChaptersQuizzed, s.ReadyChapters)
	}

	// Another user sees only their own figures, and none before their first quiz.
	other := h.client(t)
	readyChapter(t, other, "eve@example.com", "Other", 5)
	o := getDashboard(t, other)
	if o.ChapterCount != 1 || o.Stats.AverageScorePct != nil || o.Stats.AttemptCount != 0 || o.Stats.ReadyChapters != 1 {
		t.Errorf("other user's dashboard: %+v", o)
	}
}

func TestDashboardByTopic(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c) // "Cells topic": Cells and Genes ready, Broken failed

	quiz(t, c, map[string]any{"chapterId": cellsID}, 5, false)
	h.clock.Advance(time.Minute)
	quiz(t, c, map[string]any{"chapterId": cellsID}, 5, true)  // Cells' last score: 100%
	quiz(t, c, map[string]any{"chapterId": genesID}, 5, false) // Genes' last score: 0%
	quiz(t, c, map[string]any{"topicId": topicID}, 15, true)   // a topic quiz sets no chapter's last score

	// A deleted chapter's score no longer counts, and a deleted topic is not listed.
	oldID := uploadToTopic(t, c, topicID, "Old", 5)
	quiz(t, c, map[string]any{"chapterId": oldID}, 5, true)
	if code := c.do("DELETE", "/api/chapters/"+oldID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete chapter: %d", code)
	}
	goneID := uploadChapter(t, c, "Gone", 5)
	if code := c.do("DELETE", "/api/topics/"+topicOf(t, c, goneID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete topic: %d", code)
	}
	var algebra map[string]any
	c.do("POST", "/api/topics", map[string]string{"name": "Algebra"}, &algebra)

	d := getDashboard(t, c)
	if len(d.Topics) != 2 {
		t.Fatalf("topics = %+v, want Algebra and Cells topic", d.Topics)
	}
	a, b := d.Topics[0], d.Topics[1]
	if a.ID != algebra["id"] || a.Name != "Algebra" || a.ReadyChapters != 0 || a.ChaptersQuizzed != 0 || a.AverageLastScorePct != nil {
		t.Errorf("Algebra row = %+v, want nothing ready", a)
	}
	if b.ID != topicID || b.Name != "Cells topic" || b.ReadyChapters != 2 || b.ChaptersQuizzed != 2 || b.AverageLastScorePct == nil || *b.AverageLastScorePct != 50 {
		t.Errorf("Cells topic row = %+v (average %v), want 2 of 2 quizzed averaging 50", b, deref(b.AverageLastScorePct))
	}
}

func TestDashboardActivity(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	step := func() { h.clock.Advance(time.Minute) }

	cellsID := readyChapter(t, c, "ada@example.com", "Cells", 10)
	topicID := topicOf(t, c, cellsID)
	step()
	genesID := uploadToTopic(t, c, topicID, "Genes", 5)
	step()
	cellsQuiz := quiz(t, c, map[string]any{"chapterId": cellsID}, 5, true)
	step()
	h.queue.Err = errors.New("queue down")
	c.upload(map[string]string{"topicId": topicID, "count": "5", "title": "Broken"}, "broken.pdf", pdftest.Minimal("broken"), nil)
	h.queue.Err = nil
	var failed []map[string]any
	if c.do("GET", "/api/chapters?status=failed", nil, &failed); len(failed) != 1 {
		t.Fatalf("failed chapters: %+v", failed)
	}
	brokenID := failed[0]["id"].(string)
	step()
	topicQuiz := quiz(t, c, map[string]any{"topicId": topicID}, 15, false)
	step()
	// A deleted chapter's upload leaves the feed, but its attempt stays, as in History.
	oldID := uploadToTopic(t, c, topicID, "Old", 5)
	step()
	oldQuiz := quiz(t, c, map[string]any{"chapterId": oldID}, 5, true)
	if code := c.do("DELETE", "/api/chapters/"+oldID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete chapter: %d", code)
	}

	var got []string
	for _, a := range getDashboard(t, c).Activity {
		got = append(got, a.row())
	}
	want := []string{
		"attempt " + oldQuiz + " chapter Cells topic/Old 100% of 5",
		"attempt " + topicQuiz + " topic Cells topic/ 0% of 15",
		"upload " + brokenID + " Cells topic/Broken failed 0",
		"attempt " + cellsQuiz + " chapter Cells topic/Cells 100% of 5",
		"upload " + genesID + " Cells topic/Genes ready 5",
		"upload " + cellsID + " Cells topic/Cells ready 10",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("activity, newest first:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDashboardActivityShowsTheNewest30Items(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	cellsID := readyChapter(t, c, "ada@example.com", "Cells", 5)
	var quizzes []string
	for range 31 {
		h.clock.Advance(time.Minute)
		quizzes = append(quizzes, quiz(t, c, map[string]any{"chapterId": cellsID}, 5, true))
	}

	// 32 items: the upload and the first quiz are the oldest two.
	feed := getDashboard(t, c).Activity
	if len(feed) != 30 {
		t.Fatalf("activity has %d items, want 30", len(feed))
	}
	if feed[0].ID != quizzes[30] || feed[29].ID != quizzes[1] {
		t.Errorf("activity runs from %s to %s, want the last quiz %s to the second %s", feed[0].ID, feed[29].ID, quizzes[30], quizzes[1])
	}
}
