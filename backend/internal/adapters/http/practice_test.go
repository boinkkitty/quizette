package http_test

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
)

// drawnQuiz is the body of POST /api/quizzes/draw.
type drawnQuiz struct {
	Scope        string `json:"scope"`
	ChapterID    string `json:"chapterId"`
	ChapterTitle string `json:"chapterTitle"`
	TopicID      string `json:"topicId"`
	TopicName    string `json:"topicName"`
	Questions    []struct {
		ID           string   `json:"id"`
		Type         string   `json:"type"`
		Options      []string `json:"options"`
		Correct      []int    `json:"correct"`
		Explanation  string   `json:"explanation"`
		ChapterID    string   `json:"chapterId"`
		ChapterTitle string   `json:"chapterTitle"`
	} `json:"questions"`
}

// readyChapter signs c up and uploads a chapter with a bank of size questions,
// returning the chapter ID. The harness's synchronous queue makes it ready at once.
func readyChapter(t *testing.T, c *client, email, title string, size int) string {
	t.Helper()
	if code := c.do("POST", "/api/auth/signup", map[string]string{"email": email, "password": "correct horse"}, nil); code != http.StatusCreated {
		t.Fatalf("signup: %d", code)
	}
	return uploadChapter(t, c, title, size)
}

func uploadChapter(t *testing.T, c *client, title string, size int) string {
	t.Helper()
	var up map[string]string
	fields := map[string]string{"newTopicName": title + " topic", "count": strconv.Itoa(size), "title": title}
	if code := c.upload(fields, title+".pdf", pdftest.Minimal(title), &up); code != http.StatusAccepted {
		t.Fatalf("upload: %d", code)
	}
	return up["chapterId"]
}

// uploadToTopic uploads a chapter with a bank of size questions into an existing
// topic and returns the chapter ID.
func uploadToTopic(t *testing.T, c *client, topicID, title string, size int) string {
	t.Helper()
	var up map[string]string
	fields := map[string]string{"topicId": topicID, "count": strconv.Itoa(size), "title": title}
	if code := c.upload(fields, title+".pdf", pdftest.Minimal(title), &up); code != http.StatusAccepted {
		t.Fatalf("upload: %d", code)
	}
	return up["chapterId"]
}

// topicOf returns the topic of one of c's chapters.
func topicOf(t *testing.T, c *client, chapterID string) string {
	t.Helper()
	var ch map[string]any
	if code := c.do("GET", "/api/chapters/"+chapterID, nil, &ch); code != http.StatusOK {
		t.Fatalf("get chapter: %d", code)
	}
	return ch["topicId"].(string)
}

// biologyTopic gives c a topic with two ready chapters, Cells (10 questions, uploaded
// first) and Genes (5), and one failed chapter. It returns the topic and chapter IDs.
func biologyTopic(t *testing.T, h *harness, c *client) (topicID, cellsID, genesID string) {
	t.Helper()
	cellsID = readyChapter(t, c, "ada@example.com", "Cells", 10)
	topicID = topicOf(t, c, cellsID)
	h.clock.Advance(time.Second) // chapters split in creation order
	genesID = uploadToTopic(t, c, topicID, "Genes", 5)
	h.queue.Err = errors.New("queue down")
	var failed map[string]any
	c.upload(map[string]string{"topicId": topicID, "count": "20", "title": "Broken"}, "broken.pdf", pdftest.Minimal("broken"), &failed)
	h.queue.Err = nil
	var topic struct{ Chapters []map[string]any }
	if c.do("GET", "/api/topics/"+topicID, nil, &topic); len(topic.Chapters) != 3 {
		t.Fatalf("topic has %d chapters, want 3 with one failed", len(topic.Chapters))
	}
	return topicID, cellsID, genesID
}

func TestDrawTopicQuiz(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)

	// 5 split over 10 and 5 questions: quotas 3.33 and 1.67, so 3 from Cells, 2 from Genes.
	var q drawnQuiz
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 5}, &q); code != http.StatusOK {
		t.Fatalf("draw: %d", code)
	}
	if q.Scope != "topic" || q.TopicID != topicID || q.TopicName != "Cells topic" || len(q.Questions) != 5 {
		t.Fatalf("draw body: %+v", q)
	}
	perChapter := map[string]int{}
	for _, dq := range q.Questions {
		if dq.Correct != nil || dq.Explanation != "" {
			t.Fatalf("draw leaked answers: %+v", dq)
		}
		perChapter[dq.ChapterID]++
	}
	if perChapter[cellsID] != 3 || perChapter[genesID] != 2 || len(perChapter) != 2 {
		t.Errorf("questions per chapter = %v, want 3 from Cells and 2 from Genes", perChapter)
	}

	// The pool is the 15 ready questions: the failed chapter adds none.
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 15}, &q); code != http.StatusOK || len(q.Questions) != 15 {
		t.Errorf("draw whole pool: %d, %d questions", code, len(q.Questions))
	}
	for _, count := range []int{0, 7, 20} {
		if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": count}, nil); code != http.StatusBadRequest {
			t.Errorf("draw count %d: got %d, want 400", count, code)
		}
	}

	// A topic with no ready chapter can't start.
	var empty map[string]any
	c.do("POST", "/api/topics", map[string]string{"name": "Empty"}, &empty)
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": empty["id"], "count": 5}, nil); code != http.StatusBadRequest {
		t.Errorf("draw from a topic with no ready chapter: %d, want 400", code)
	}
	// Exactly one of chapterId and topicId names the scope.
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "chapterId": cellsID, "count": 5}, nil); code != http.StatusBadRequest {
		t.Errorf("draw with both scopes: %d, want 400", code)
	}

	other := h.client(t)
	readyChapter(t, other, "eve@example.com", "Other", 5)
	if code := other.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 5}, nil); code != http.StatusNotFound {
		t.Errorf("draw from another user's topic: %d", code)
	}
}

// scopedAttempt is an attempt response with its scope and each question's chapter.
type scopedAttempt struct {
	ID         string  `json:"id"`
	Scope      string  `json:"scope"`
	TopicID    string  `json:"topicId"`
	TopicName  string  `json:"topicName"`
	ChapterID  string  `json:"chapterId"`
	TotalScore float64 `json:"totalScore"`
	MaxScore   int     `json:"maxScore"`
	Questions  []struct {
		ID           string `json:"id"`
		ChapterID    string `json:"chapterId"`
		ChapterTitle string `json:"chapterTitle"`
	} `json:"questions"`
}

func TestSubmitTopicQuiz(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)
	elsewhereID := uploadChapter(t, c, "Atoms", 5) // in its own topic

	var q, elsewhere drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 10}, &q)
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": elsewhereID, "count": 5}, &elsewhere)
	ids := questionIDs(q)

	rejected := map[string][]string{
		"question from another topic":  append(ids[:9:9], elsewhere.Questions[0].ID),
		"count not a multiple of 5":    ids[:9],
		"count above the topic's pool": append(ids, ids...), // 20 > 15, before the duplicates count
	}
	for name, qids := range rejected {
		body := map[string]any{"topicId": topicID, "questionIds": qids, "answers": []any{}}
		if code := c.do("POST", "/api/attempts", body, nil); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}

	answers, want := stubAnswers(q)
	var res scopedAttempt
	body := map[string]any{"topicId": topicID, "questionIds": ids, "answers": answers}
	if code := c.do("POST", "/api/attempts", body, &res); code != http.StatusCreated {
		t.Fatalf("submit: %d", code)
	}
	if res.Scope != "topic" || res.TopicID != topicID || res.TopicName != "Cells topic" || res.ChapterID != "" || res.TotalScore != want || res.MaxScore != 10 {
		t.Fatalf("result: %+v (want score %v)", res, want)
	}
	titles := map[string]string{cellsID: "Cells", genesID: "Genes"}
	for i, rq := range res.Questions {
		if rq.ID != ids[i] || titles[rq.ChapterID] == "" || rq.ChapterTitle != titles[rq.ChapterID] {
			t.Errorf("result question %d: %+v, want %s with its source chapter", i, rq, ids[i])
		}
	}

	var got scopedAttempt
	if code := c.do("GET", "/api/attempts/"+res.ID, nil, &got); code != http.StatusOK || got.Scope != "topic" || len(got.Questions) != 10 || got.Questions[0].ChapterID != res.Questions[0].ChapterID {
		t.Fatalf("get attempt: %d %+v", code, got)
	}
	var hist []map[string]any
	c.do("GET", "/api/attempts", nil, &hist)
	if len(hist) != 1 || hist[0]["scope"] != "topic" || hist[0]["topicId"] != topicID || hist[0]["topicName"] != "Cells topic" {
		t.Fatalf("history: %+v", hist)
	}
}

// submitQuiz submits every drawn question of q to the scope in body, answered
// correctly if correct is set and left unanswered otherwise.
func submitQuiz(t *testing.T, c *client, body map[string]any, q drawnQuiz, correct bool) {
	t.Helper()
	answers := []map[string]any{}
	for _, dq := range q.Questions {
		if correct {
			sel := map[string][]int{"mcq": {1}, "true_false": {0}, "mrq": {0, 2}}[dq.Type]
			answers = append(answers, map[string]any{"questionId": dq.ID, "selected": sel})
		}
	}
	body["questionIds"], body["answers"] = questionIDs(q), answers
	if code := c.do("POST", "/api/attempts", body, nil); code != http.StatusCreated {
		t.Fatalf("submit %v: %d", body, code)
	}
}

func TestPerformanceIsKeptSeparateByScope(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)

	var q drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": cellsID, "count": 5}, &q)
	submitQuiz(t, c, map[string]any{"chapterId": cellsID}, q, false) // Cells chapter quiz: 0%
	h.clock.Advance(time.Minute)
	c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 15}, &q)
	submitQuiz(t, c, map[string]any{"topicId": topicID}, q, true) // topic quiz: 100%, covers every chapter
	h.clock.Advance(time.Minute)
	c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 5}, &q)
	submitQuiz(t, c, map[string]any{"topicId": topicID}, q, false) // topic quiz: 0%, the last one

	var topics []struct {
		ID           string   `json:"id"`
		AttemptCount int      `json:"attemptCount"`
		LastScorePct *float64 `json:"lastScorePct"`
		BestScorePct *float64 `json:"bestScorePct"`
	}
	c.do("GET", "/api/topics", nil, &topics)
	if len(topics) != 1 || topics[0].ID != topicID {
		t.Fatalf("topics: %+v", topics)
	}
	if got := topics[0]; got.AttemptCount != 2 || got.LastScorePct == nil || *got.LastScorePct != 0 || got.BestScorePct == nil || *got.BestScorePct != 100 {
		t.Errorf("topic performance = count %d, last %v, best %v; want 2, 0, 100", got.AttemptCount, deref(got.LastScorePct), deref(got.BestScorePct))
	}

	var chapters []struct {
		ID           string   `json:"id"`
		AttemptCount int      `json:"attemptCount"`
		BestScorePct *float64 `json:"bestScorePct"`
	}
	c.do("GET", "/api/chapters?topicId="+topicID, nil, &chapters)
	stats := map[string]string{}
	for _, ch := range chapters {
		stats[ch.ID] = fmt.Sprintf("%d %v", ch.AttemptCount, deref(ch.BestScorePct))
	}
	// The perfect topic quiz must not raise either chapter's best score.
	if stats[cellsID] != "1 0" || stats[genesID] != "0 <nil>" {
		t.Errorf("chapter performance = Cells %q, Genes %q; want \"1 0\" and \"0 <nil>\"", stats[cellsID], stats[genesID])
	}
	var hist []map[string]any
	if c.do("GET", "/api/chapters/"+cellsID+"/attempts", nil, &hist); len(hist) != 1 {
		t.Errorf("Cells attempt history has %d attempts, want only its chapter quiz", len(hist))
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestDrawQuiz(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	chapterID := readyChapter(t, c, "ada@example.com", "Cells", 10)

	var q drawnQuiz
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": 5}, &q); code != http.StatusOK {
		t.Fatalf("draw: %d", code)
	}
	if q.ChapterID != chapterID || q.ChapterTitle != "Cells" || len(q.Questions) != 5 {
		t.Fatalf("draw body: %+v", q)
	}
	seen := map[string]bool{}
	for _, dq := range q.Questions {
		if dq.Correct != nil || dq.Explanation != "" {
			t.Fatalf("draw leaked answers: %+v", dq)
		}
		if seen[dq.ID] {
			t.Fatalf("question %s drawn twice", dq.ID)
		}
		seen[dq.ID] = true
	}

	for _, count := range []int{0, 3, 7, 15} {
		if code := c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": count}, nil); code != http.StatusBadRequest {
			t.Errorf("draw count %d: got %d, want 400", count, code)
		}
	}
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": 10}, nil); code != http.StatusOK {
		t.Errorf("draw whole bank: %d", code)
	}

	other := h.client(t)
	readyChapter(t, other, "eve@example.com", "Other", 5)
	if code := other.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": 5}, nil); code != http.StatusNotFound {
		t.Errorf("draw from another user's chapter: %d", code)
	}
}

// stubAnswers answers each drawn question the way the stub LLM grades it: MCQ option 1
// (correct), True/False 0 (correct) and MRQ only A (half credit). It returns the
// answers to submit and the score they should earn.
func stubAnswers(q drawnQuiz) (answers []map[string]any, want float64) {
	for _, dq := range q.Questions {
		sel := map[string][]int{"mcq": {1}, "true_false": {0}, "mrq": {0}}[dq.Type]
		want += map[string]float64{"mcq": 1, "true_false": 1, "mrq": 0.5}[dq.Type]
		answers = append(answers, map[string]any{"questionId": dq.ID, "selected": sel})
	}
	return answers, want
}

func questionIDs(q drawnQuiz) []string {
	out := make([]string, len(q.Questions))
	for i, dq := range q.Questions {
		out[i] = dq.ID
	}
	return out
}

func TestSubmitQuizCreatesGradedAttempt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	chapterID := readyChapter(t, c, "ada@example.com", "Cells", 10)

	var q drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": 5}, &q)
	answers, want := stubAnswers(q)
	answers = answers[1:] // first question left unanswered: scores 0
	want -= map[string]float64{"mcq": 1, "true_false": 1, "mrq": 0.5}[q.Questions[0].Type]

	var res attempt
	body := map[string]any{"chapterId": chapterID, "questionIds": questionIDs(q), "answers": answers}
	if code := c.do("POST", "/api/attempts", body, &res); code != http.StatusCreated {
		t.Fatalf("submit: %d", code)
	}
	if res.ID == "" || res.TotalScore != want || res.MaxScore != 5 || len(res.Answers) != 5 || res.Answers[0].Points != 0 {
		t.Fatalf("result: %+v (want score %v)", res, want)
	}
	for i, rq := range res.Questions {
		if rq.ID != q.Questions[i].ID || rq.Correct == nil {
			t.Fatalf("result question %d: %+v, want %s in draw order with answers", i, rq, q.Questions[i].ID)
		}
	}

	var got attempt
	if code := c.do("GET", "/api/attempts/"+res.ID, nil, &got); code != http.StatusOK || got.TotalScore != want {
		t.Fatalf("get attempt: %d %+v", code, got)
	}
	var hist []map[string]any
	c.do("GET", "/api/attempts", nil, &hist)
	if len(hist) != 1 || hist[0]["id"] != res.ID || hist[0]["submittedAt"] == nil {
		t.Fatalf("history: %+v", hist)
	}
	var chapters []map[string]any
	c.do("GET", "/api/chapters", nil, &chapters)
	if len(chapters) != 1 || chapters[0]["attemptCount"] != float64(1) {
		t.Fatalf("chapter stats: %+v", chapters)
	}
}

func TestSubmitQuizRejectsInvalidDraws(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	chapterID := readyChapter(t, c, "ada@example.com", "Cells", 10)
	otherChapterID := uploadChapter(t, c, "Genes", 5)

	var q, other drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": chapterID, "count": 5}, &q)
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": otherChapterID, "count": 5}, &other)
	ids := questionIDs(q)

	tests := map[string][]string{
		"question from another chapter": append(ids[:4:4], other.Questions[0].ID),
		"count not a multiple of 5":     ids[:4],
		"no questions":                  {},
	}
	for name, qids := range tests {
		body := map[string]any{"chapterId": chapterID, "questionIds": qids, "answers": []any{}}
		if code := c.do("POST", "/api/attempts", body, nil); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}

	eve := h.client(t)
	readyChapter(t, eve, "eve@example.com", "Eve", 5)
	if code := eve.do("POST", "/api/attempts", map[string]any{"chapterId": chapterID, "questionIds": ids, "answers": []any{}}, nil); code != http.StatusNotFound {
		t.Errorf("submit to another user's chapter: %d", code)
	}

	var hist []map[string]any
	c.do("GET", "/api/attempts", nil, &hist)
	if len(hist) != 0 {
		t.Fatalf("rejected submits were recorded: %+v", hist)
	}

	// The in-progress attempt endpoints are gone (ADR 0011).
	if code := c.do("POST", "/api/chapters/"+chapterID+"/attempts", map[string]int{"count": 5}, nil); code != http.StatusMethodNotAllowed {
		t.Errorf("start attempt endpoint: %d", code)
	}
	if code := c.do("POST", "/api/attempts/x/submit", map[string]any{}, nil); code != http.StatusNotFound {
		t.Errorf("submit attempt endpoint: %d", code)
	}
}
