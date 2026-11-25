package http_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
)

// drawnQuiz is the body of POST /api/quizzes/draw.
type drawnQuiz struct {
	ChapterID    string `json:"chapterId"`
	ChapterTitle string `json:"chapterTitle"`
	Questions    []struct {
		ID          string   `json:"id"`
		Type        string   `json:"type"`
		Options     []string `json:"options"`
		Correct     []int    `json:"correct"`
		Explanation string   `json:"explanation"`
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
