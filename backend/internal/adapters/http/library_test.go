package http_test

import (
	"net/http"
	"testing"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
)

// deletedAttempt is an attempt response with the deleted-content flags.
type deletedAttempt struct {
	ID           string  `json:"id"`
	TotalScore   float64 `json:"totalScore"`
	ScopeDeleted bool    `json:"scopeDeleted"`
	Questions    []struct {
		ID             string `json:"id"`
		ChapterID      string `json:"chapterId"`
		ChapterTitle   string `json:"chapterTitle"`
		ChapterDeleted bool   `json:"chapterDeleted"`
	} `json:"questions"`
}

// listedChapterIDs returns the IDs in c's chapter listing.
func listedChapterIDs(t *testing.T, c *client) map[string]bool {
	t.Helper()
	var chapters []map[string]any
	if code := c.do("GET", "/api/chapters", nil, &chapters); code != http.StatusOK {
		t.Fatalf("list chapters: %d", code)
	}
	out := map[string]bool{}
	for _, ch := range chapters {
		out[ch["id"].(string)] = true
	}
	return out
}

func TestDeleteChapterWithAttemptsKeepsItForTheAttempts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)

	var q drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": cellsID, "count": 5}, &q)
	answers, want := stubAnswers(q)
	var res deletedAttempt
	if code := c.do("POST", "/api/attempts", map[string]any{"chapterId": cellsID, "questionIds": questionIDs(q), "answers": answers}, &res); code != http.StatusCreated {
		t.Fatalf("submit: %d", code)
	}

	if code := c.do("DELETE", "/api/chapters/"+cellsID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete chapter: %d", code)
	}

	// The deleted chapter is hidden from listings, detail and draws.
	if listed := listedChapterIDs(t, c); listed[cellsID] || !listed[genesID] {
		t.Errorf("chapter listing after delete: %v, want Genes but not Cells", listed)
	}
	if code := c.do("GET", "/api/chapters/"+cellsID, nil, nil); code != http.StatusNotFound {
		t.Errorf("get deleted chapter: %d, want 404", code)
	}
	var topic struct{ Chapters []map[string]any }
	c.do("GET", "/api/topics/"+topicID, nil, &topic)
	for _, ch := range topic.Chapters {
		if ch["id"] == cellsID {
			t.Errorf("topic detail still lists the deleted chapter")
		}
	}
	var topics []map[string]any
	c.do("GET", "/api/topics", nil, &topics)
	if len(topics) != 1 || topics[0]["chapterCount"] != float64(2) {
		t.Errorf("topic list: %+v, want chapterCount 2 (Genes and the failed chapter)", topics)
	}
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": cellsID, "count": 5}, nil); code != http.StatusNotFound {
		t.Errorf("draw from deleted chapter: %d, want 404", code)
	}
	// The topic's pool is now Genes alone.
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 10}, nil); code != http.StatusBadRequest {
		t.Errorf("draw 10 from a topic with 5 live questions: %d, want 400", code)
	}

	// The past attempt keeps its score and questions, flagged as deleted.
	var got deletedAttempt
	if code := c.do("GET", "/api/attempts/"+res.ID, nil, &got); code != http.StatusOK {
		t.Fatalf("get attempt: %d", code)
	}
	if got.TotalScore != want || !got.ScopeDeleted || len(got.Questions) != 5 {
		t.Fatalf("attempt after delete: %+v, want score %v, scope deleted and 5 questions", got, want)
	}
	for i, gq := range got.Questions {
		if gq.ID != q.Questions[i].ID || gq.ChapterID != cellsID || gq.ChapterTitle != "Cells" || !gq.ChapterDeleted {
			t.Errorf("question %d: %+v, want %s from deleted chapter Cells", i, gq, q.Questions[i].ID)
		}
	}
	var hist []map[string]any
	c.do("GET", "/api/attempts", nil, &hist)
	if len(hist) != 1 || hist[0]["scopeDeleted"] != true || hist[0]["chapterTitle"] != "Cells" {
		t.Errorf("history: %+v, want the attempt flagged scopeDeleted", hist)
	}
}

func TestDeleteTopicWithAttemptsKeepsItForTheAttempts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)

	var q drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 10}, &q)
	var res deletedAttempt
	if code := c.do("POST", "/api/attempts", map[string]any{"topicId": topicID, "questionIds": questionIDs(q), "answers": []any{}}, &res); code != http.StatusCreated {
		t.Fatalf("submit: %d", code)
	}

	if code := c.do("DELETE", "/api/topics/"+topicID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete topic: %d", code)
	}

	// The topic and its chapters are hidden.
	var topics []map[string]any
	if c.do("GET", "/api/topics", nil, &topics); len(topics) != 0 {
		t.Errorf("topic list after delete: %+v", topics)
	}
	if code := c.do("GET", "/api/topics/"+topicID, nil, nil); code != http.StatusNotFound {
		t.Errorf("get deleted topic: %d, want 404", code)
	}
	if listed := listedChapterIDs(t, c); len(listed) != 0 {
		t.Errorf("chapter listing after deleting their topic: %v", listed)
	}
	for _, id := range []string{cellsID, genesID} {
		if code := c.do("GET", "/api/chapters/"+id, nil, nil); code != http.StatusNotFound {
			t.Errorf("get chapter of deleted topic: %d, want 404", code)
		}
	}
	if code := c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 5}, nil); code != http.StatusNotFound {
		t.Errorf("draw from deleted topic: %d, want 404", code)
	}
	upload := map[string]string{"topicId": topicID, "count": "5", "title": "Late"}
	if code := c.upload(upload, "late.pdf", pdftest.Minimal("late"), nil); code != http.StatusNotFound {
		t.Errorf("upload to deleted topic: %d, want 404", code)
	}

	// The past attempt keeps its questions, flagged as deleted.
	var got deletedAttempt
	if code := c.do("GET", "/api/attempts/"+res.ID, nil, &got); code != http.StatusOK {
		t.Fatalf("get attempt: %d", code)
	}
	if !got.ScopeDeleted || len(got.Questions) != 10 {
		t.Fatalf("attempt after delete: %+v, want scope deleted and 10 questions", got)
	}
	for i, gq := range got.Questions {
		if !gq.ChapterDeleted || gq.ChapterTitle == "" {
			t.Errorf("question %d: %+v, want its chapter's title, flagged deleted", i, gq)
		}
	}
	var hist []map[string]any
	c.do("GET", "/api/attempts", nil, &hist)
	if len(hist) != 1 || hist[0]["scopeDeleted"] != true || hist[0]["topicName"] != "Cells topic" {
		t.Errorf("history: %+v, want the attempt flagged scopeDeleted", hist)
	}

	// The deleted topic's name is free.
	var fresh map[string]any
	if code := c.do("POST", "/api/topics", map[string]string{"name": "Cells topic"}, &fresh); code != http.StatusCreated {
		t.Fatalf("create topic reusing a deleted topic's name: %d", code)
	}
	if code := c.do("PATCH", "/api/topics/"+fresh["id"].(string), map[string]string{"name": "cells TOPIC"}, nil); code != http.StatusOK {
		t.Errorf("rename to a deleted topic's name: %d", code)
	}
	var detail struct{ Chapters []map[string]any }
	if c.do("GET", "/api/topics/"+fresh["id"].(string), nil, &detail); len(detail.Chapters) != 0 {
		t.Errorf("new topic inherited chapters: %+v", detail.Chapters)
	}
}

func TestDeleteWithoutAttemptsRemovesContent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)

	if code := c.do("DELETE", "/api/chapters/"+cellsID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete chapter: %d", code)
	}
	if code := c.do("GET", "/api/chapters/"+cellsID, nil, nil); code != http.StatusNotFound {
		t.Errorf("get deleted chapter: %d, want 404", code)
	}
	if code := c.do("DELETE", "/api/chapters/"+cellsID, nil, nil); code != http.StatusNotFound {
		t.Errorf("delete chapter twice: %d, want 404", code)
	}
	if code := c.do("DELETE", "/api/topics/"+topicID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete topic: %d", code)
	}
	if code := c.do("GET", "/api/chapters/"+genesID, nil, nil); code != http.StatusNotFound {
		t.Errorf("get chapter of deleted topic: %d, want 404", code)
	}
	if code := c.do("DELETE", "/api/topics/"+topicID, nil, nil); code != http.StatusNotFound {
		t.Errorf("delete topic twice: %d, want 404", code)
	}
	if code := c.do("POST", "/api/topics", map[string]string{"name": "Cells topic"}, nil); code != http.StatusCreated {
		t.Errorf("reuse a removed topic's name: %d", code)
	}
}

func TestSubmitNamingDeletedQuestionsIsRejected(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	topicID, cellsID, genesID := biologyTopic(t, h, c)
	atomsID := uploadChapter(t, c, "Atoms", 5)

	// Quizzes drawn before their content is deleted.
	var cellsQuiz, genesQuiz, atomsQuiz, topicQuiz drawnQuiz
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": cellsID, "count": 5}, &cellsQuiz)
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": genesID, "count": 5}, &genesQuiz)
	c.do("POST", "/api/quizzes/draw", map[string]any{"chapterId": atomsID, "count": 5}, &atomsQuiz)
	c.do("POST", "/api/quizzes/draw", map[string]any{"topicId": topicID, "count": 15}, &topicQuiz)

	// Cells has an attempt, so it is kept as deleted; Atoms has none, so it is removed.
	submitQuiz(t, c, map[string]any{"chapterId": cellsID}, cellsQuiz, true)
	for _, id := range []string{cellsID, atomsID} {
		if code := c.do("DELETE", "/api/chapters/"+id, nil, nil); code != http.StatusNoContent {
			t.Fatalf("delete chapter: %d", code)
		}
	}

	tests := map[string]struct {
		scope map[string]any
		quiz  drawnQuiz
	}{
		"chapter quiz of a kept deleted chapter": {map[string]any{"chapterId": cellsID}, cellsQuiz},
		"chapter quiz of a removed chapter":      {map[string]any{"chapterId": atomsID}, atomsQuiz},
		"topic quiz with a deleted chapter":      {map[string]any{"topicId": topicID}, topicQuiz},
	}
	for name, tt := range tests {
		body := map[string]any{"questionIds": questionIDs(tt.quiz), "answers": []any{}}
		for k, v := range tt.scope {
			body[k] = v
		}
		var e struct{ Error string }
		if code := c.do("POST", "/api/attempts", body, &e); code != http.StatusConflict || e.Error != "content_deleted" {
			t.Errorf("%s: got %d %q, want 409 content_deleted", name, code, e.Error)
		}
	}

	// A quiz over content that is still there submits as usual.
	submitQuiz(t, c, map[string]any{"chapterId": genesID}, genesQuiz, true)
	var hist []map[string]any
	if c.do("GET", "/api/attempts", nil, &hist); len(hist) != 2 {
		t.Errorf("history has %d attempts, want the two accepted submits", len(hist))
	}
}
