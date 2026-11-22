package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	qhttp "github.com/boinkkitty/quizette/backend/internal/adapters/http"
	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/stub"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/app/apptest"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type harness struct {
	srv   *httptest.Server
	logs  *bytes.Buffer
	clock *apptest.Clock
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := apptest.NewStore()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	clock := apptest.NewClock(time.Now())
	deps := app.Deps{Repos: store.Repos(), UoW: store, Clock: clock, IDs: &apptest.IDs{}, Logger: logger}
	queue := &apptest.SyncQueue{}
	gen := &app.GenerationService{Deps: deps, Generator: stub.Generator{}, Queue: queue, Config: app.GenerationConfig{}}
	queue.Handler = gen.ProcessJob
	ids := &app.IdentityService{Deps: deps, Hasher: apptest.Hasher{}, Providers: map[string]app.OAuthProvider{
		"github": apptest.OAuthProvider{ProviderName: "github", Identity: domain.OAuthIdentity{ProviderUserID: "1", Email: "gh@example.com", EmailVerified: true}},
	}}
	h := qhttp.NewRouter(qhttp.Config{
		Identity: ids, Library: &app.LibraryService{Deps: deps}, Generation: gen, Practice: &app.PracticeService{Deps: deps},
		Logger: logger, PublicURL: "http://app.test",
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &harness{srv: srv, logs: logs, clock: clock}
}

// client mimics the SPA: the cookie jar holds the session cookie and every
// request carries the CSRF header.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (h *harness) client(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: h.srv.URL, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(context.Background(), method, c.base+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(qhttp.CSRFHeader, "quizette")
	return c.send(req, out)
}

func (c *client) send(req *http.Request, out any) int {
	c.t.Helper()
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

// sessionCookie returns the client's current session cookie, if any.
func (c *client) sessionCookie() *http.Cookie {
	u, _ := url.Parse(c.base + "/")
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "qz_session" {
			return ck
		}
	}
	return nil
}

func (c *client) upload(fields map[string]string, filename string, data []byte, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if data != nil {
		fw, _ := mw.CreateFormFile("file", filename)
		_, _ = fw.Write(data)
	}
	_ = mw.Close()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, c.base+"/api/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(qhttp.CSRFHeader, "quizette")
	return c.send(req, out)
}

type attempt struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	TotalScore float64 `json:"totalScore"`
	MaxScore   int     `json:"maxScore"`
	Questions  []struct {
		ID      string   `json:"id"`
		Type    string   `json:"type"`
		Options []string `json:"options"`
		Correct []int    `json:"correct"`
	} `json:"questions"`
	Answers []struct {
		Points float64 `json:"points"`
	} `json:"answers"`
}

func TestEndToEndFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)

	if code := c.do("GET", "/api/topics", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated topics: %d", code)
	}
	var signup map[string]string
	if code := c.do("POST", "/api/auth/signup", map[string]string{"email": "ada@example.com", "password": "correct horse"}, &signup); code != http.StatusCreated {
		t.Fatalf("signup: %d", code)
	}
	if signup["email"] != "ada@example.com" {
		t.Fatalf("signup body: %+v", signup)
	}
	var user map[string]string
	if code := c.do("GET", "/api/auth/me", nil, &user); code != http.StatusOK || user["email"] != "ada@example.com" {
		t.Fatalf("me: %d %v", code, user)
	}

	// Validation: wrong count, not a PDF, missing file.
	var e map[string]any
	if code := c.upload(map[string]string{"newTopicName": "Bio", "count": "7"}, "a.pdf", pdftest.Minimal("x"), &e); code != http.StatusBadRequest {
		t.Fatalf("bad count: %d %v", code, e)
	}
	if code := c.upload(map[string]string{"newTopicName": "Bio", "count": "5"}, "a.txt", []byte("hello"), nil); code != http.StatusBadRequest {
		t.Fatalf("not pdf: %d", code)
	}
	if code := c.upload(map[string]string{"newTopicName": "Bio", "count": "5"}, "", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("no file: %d", code)
	}

	var up map[string]string
	if code := c.upload(map[string]string{"newTopicName": "Bio", "count": "10", "title": "Cells"}, "cells.pdf", pdftest.Minimal("cells"), &up); code != http.StatusAccepted {
		t.Fatalf("upload: %d", code)
	}
	var job map[string]any
	if code := c.do("GET", "/api/jobs/"+up["jobId"], nil, &job); code != http.StatusOK || job["status"] != "succeeded" {
		t.Fatalf("job: %d %v", code, job)
	}
	var topic struct {
		Chapters []map[string]any `json:"chapters"`
	}
	c.do("GET", "/api/topics/"+up["topicId"], nil, &topic)
	if len(topic.Chapters) != 1 || topic.Chapters[0]["status"] != "ready" || topic.Chapters[0]["questionCount"] != float64(10) {
		t.Fatalf("topic chapters: %+v", topic.Chapters)
	}

	if code := c.do("POST", "/api/chapters/"+up["chapterId"]+"/attempts", map[string]int{"count": 15}, nil); code != http.StatusBadRequest {
		t.Fatalf("count > bank: %d", code)
	}
	var a attempt
	if code := c.do("POST", "/api/chapters/"+up["chapterId"]+"/attempts", map[string]int{"count": 5}, &a); code != http.StatusCreated {
		t.Fatalf("start: %d", code)
	}
	if len(a.Questions) != 5 {
		t.Fatalf("questions: %d", len(a.Questions))
	}
	for _, q := range a.Questions {
		if q.Correct != nil {
			t.Fatal("answers leaked before submission")
		}
	}

	// Answer: MCQ option 1 (correct for the stub), TF 0 (correct), MRQ only A (half).
	answers := []map[string]any{}
	want := 0.0
	for _, q := range a.Questions {
		sel := map[string][]int{"mcq": {1}, "true_false": {0}, "mrq": {0}}[q.Type]
		want += map[string]float64{"mcq": 1, "true_false": 1, "mrq": 0.5}[q.Type]
		answers = append(answers, map[string]any{"questionId": q.ID, "selected": sel})
	}
	var res attempt
	if code := c.do("POST", "/api/attempts/"+a.ID+"/submit", map[string]any{"answers": answers}, &res); code != http.StatusOK {
		t.Fatalf("submit: %d", code)
	}
	if res.Status != "submitted" || res.TotalScore != want || res.MaxScore != 5 || len(res.Answers) != 5 || res.Questions[0].Correct == nil {
		t.Fatalf("result: %+v (want score %v)", res, want)
	}
	if code := c.do("POST", "/api/attempts/"+a.ID+"/submit", map[string]any{"answers": answers}, nil); code != http.StatusConflict {
		t.Fatalf("resubmit: %d", code)
	}
	var hist []map[string]any
	c.do("GET", "/api/chapters/"+up["chapterId"]+"/attempts", nil, &hist)
	if len(hist) != 1 {
		t.Fatalf("history: %v", hist)
	}

	// Owner-wide lists behind the Quizzes and History pages.
	var chapters []map[string]any
	if code := c.do("GET", "/api/chapters", nil, &chapters); code != http.StatusOK || len(chapters) != 1 ||
		chapters[0]["topicName"] != "Bio" || chapters[0]["title"] != "Cells" || chapters[0]["attemptCount"] != float64(1) {
		t.Fatalf("all chapters: %d %+v", code, chapters)
	}
	var ready, failed, inTopic, otherTopic []map[string]any
	c.do("GET", "/api/chapters?status=ready", nil, &ready)
	c.do("GET", "/api/chapters?status=failed", nil, &failed)
	c.do("GET", "/api/chapters?status=ready&topicId="+up["topicId"], nil, &inTopic)
	c.do("GET", "/api/chapters?topicId=0190f5e2-0000-7000-8000-000000000000", nil, &otherTopic)
	if len(ready) != 1 || len(failed) != 0 || len(inTopic) != 1 || len(otherTopic) != 0 {
		t.Fatalf("chapter filters: ready=%v failed=%v inTopic=%v otherTopic=%v", ready, failed, inTopic, otherTopic)
	}
	if code := c.do("GET", "/api/chapters?status=bogus", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad status filter: %d", code)
	}
	var all []map[string]any
	if code := c.do("GET", "/api/attempts", nil, &all); code != http.StatusOK || len(all) != 1 ||
		all[0]["id"] != a.ID || all[0]["chapterTitle"] != "Cells" || all[0]["topicName"] != "Bio" || all[0]["topicId"] != up["topicId"] {
		t.Fatalf("all attempts: %d %+v", code, all)
	}

	// Another user cannot see this chapter.
	other := h.client(t)
	other.do("POST", "/api/auth/signup", map[string]string{"email": "eve@example.com", "password": "password123"}, nil)
	if code := other.do("GET", "/api/chapters/"+up["chapterId"], nil, nil); code != http.StatusNotFound {
		t.Fatalf("other user chapter: %d", code)
	}
	if code := other.do("GET", "/api/attempts/"+a.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("other user attempt: %d", code)
	}
	var otherChapters, otherAttempts []map[string]any
	other.do("GET", "/api/chapters", nil, &otherChapters)
	other.do("GET", "/api/attempts", nil, &otherAttempts)
	if len(otherChapters) != 0 || len(otherAttempts) != 0 {
		t.Fatalf("other user sees lists: %v %v", otherChapters, otherAttempts)
	}

	if code := c.do("POST", "/api/auth/logout", nil, nil); code != http.StatusNoContent {
		t.Fatalf("logout: %d", code)
	}
	if code := c.do("GET", "/api/auth/me", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", code)
	}

	// Request logs: one line per request with request_id, never document content.
	logs := h.logs.String()
	if !strings.Contains(logs, `"msg":"http.request"`) || !strings.Contains(logs, `"request_id"`) || !strings.Contains(logs, `"route":"/api/documents"`) {
		t.Errorf("request logs missing fields:\n%.500s", logs)
	}
	if strings.Contains(logs, "correct horse") {
		t.Error("password leaked into logs")
	}
}

func TestCSRFGuard(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	c.do("POST", "/api/auth/signup", map[string]string{"email": "ada@example.com", "password": "correct horse"}, nil)

	// Every mutating API route is cookie-authenticated, so each must reject a
	// request without the CSRF header even when the session cookie is present.
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/auth/signup"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodPost, "/api/topics"},
		{http.MethodPost, "/api/documents"},
		{http.MethodDelete, "/api/chapters/x"},
	} {
		req, _ := http.NewRequestWithContext(context.Background(), tc.method, h.srv.URL+tc.path, strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: want 403, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	if code := c.do("GET", "/api/topics", nil, nil); code != http.StatusOK {
		t.Errorf("GET needs no CSRF header: %d", code)
	}
}

func TestSessionCookie(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, h.srv.URL+"/api/auth/signup", strings.NewReader(`{"email":"ada@example.com","password":"correct horse"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(qhttp.CSRFHeader, "quizette")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var set *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "qz_session" {
			set = ck
		}
	}
	if set == nil || !set.HttpOnly || set.SameSite != http.SameSiteLaxMode || set.Path != "/" || set.Value == "" {
		t.Fatalf("session cookie attributes: %+v", set)
	}
	if ttl := time.Until(set.Expires); ttl < app.SessionTTL-time.Hour || ttl > app.SessionTTL+time.Hour {
		t.Errorf("session cookie expiry %v, want about %v", ttl, app.SessionTTL)
	}

	// Logout ends the session on the server: a copied cookie stops working.
	stolen := h.client(t)
	u, _ := url.Parse(h.srv.URL + "/")
	stolen.http.Jar.SetCookies(u, []*http.Cookie{{Name: "qz_session", Value: set.Value, Path: "/"}})
	if code := stolen.do("GET", "/api/auth/me", nil, nil); code != http.StatusOK {
		t.Fatalf("copied cookie before logout: %d", code)
	}
	if code := c.do("POST", "/api/auth/logout", nil, nil); code != http.StatusNoContent {
		t.Fatalf("logout: %d", code)
	}
	if c.sessionCookie() != nil {
		t.Error("logout did not clear the session cookie")
	}
	if code := stolen.do("GET", "/api/auth/me", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("copied cookie after logout: %d", code)
	}
	if stolen.sessionCookie() != nil {
		t.Error("an invalid session cookie should be cleared")
	}
}

func TestSessionExpiry(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	c.do("POST", "/api/auth/signup", map[string]string{"email": "ada@example.com", "password": "correct horse"}, nil)
	if code := c.do("GET", "/api/topics", nil, nil); code != http.StatusOK {
		t.Fatalf("before expiry: %d", code)
	}
	h.clock.Advance(app.SessionTTL + time.Second)
	if code := c.do("GET", "/api/topics", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("after expiry: %d", code)
	}
}

func TestOAuthFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, h.srv.URL+"/api/auth/github/start", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusFound || !strings.HasPrefix(loc, "https://auth.example/github?state=") {
		t.Fatalf("start: %d %s", resp.StatusCode, loc)
	}

	// Wrong state is rejected.
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, h.srv.URL+"/api/auth/github/callback?code=x&state=wrong", nil)
	resp, _ = c.http.Do(req)
	resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Location"), "http://app.test/login?error=") {
		t.Fatalf("wrong state: %s", resp.Header.Get("Location"))
	}

	// Restart and complete properly.
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, h.srv.URL+"/api/auth/github/start", nil)
	resp, _ = c.http.Do(req)
	resp.Body.Close()
	state := strings.TrimPrefix(resp.Header.Get("Location"), "https://auth.example/github?state=")
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, h.srv.URL+"/api/auth/github/callback?code=x&state="+state, nil)
	resp, _ = c.http.Do(req)
	resp.Body.Close()
	if resp.Header.Get("Location") != "http://app.test/" {
		t.Fatalf("callback: %s", resp.Header.Get("Location"))
	}
	var me map[string]string
	if code := c.do("GET", "/api/auth/me", nil, &me); code != http.StatusOK || me["email"] != "gh@example.com" {
		t.Fatalf("me: %d %v", code, me)
	}
}

func TestUploadTooLarge(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	c := h.client(t)
	c.do("POST", "/api/auth/signup", map[string]string{"email": "big@example.com", "password": "password123"}, nil)
	big := append([]byte("%PDF-1.4\n"), make([]byte, domain.MaxDocumentBytes+10)...)
	var e map[string]any
	if code := c.upload(map[string]string{"newTopicName": "T", "count": "5"}, "big.pdf", big, &e); code != http.StatusBadRequest {
		t.Fatalf("too large: %d %v", code, e)
	}
}
