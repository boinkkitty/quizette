package openrouter_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/openrouter"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/ledongthuc"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type captured struct {
	mu     sync.Mutex
	path   string
	header http.Header
	body   map[string]any
}

// fakeOpenRouter answers every request with status and body, recording the request.
func fakeOpenRouter(t *testing.T, status int, body any) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.path, c.header = r.URL.Path, r.Header.Clone()
		_ = json.Unmarshal(raw, &c.body)
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func completion(model, content string) map[string]any {
	return map[string]any{
		"id": "gen-1", "model": model,
		"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}},
		"usage": map[string]any{
			"prompt_tokens": 1200, "completion_tokens": 1100, "total_tokens": 2300,
			"completion_tokens_details": map[string]any{"reasoning_tokens": 300},
		},
	}
}

func questionsJSON(n int) string {
	qs := make([]app.GeneratedQuestion, n)
	for i := range qs {
		qs[i] = app.GeneratedQuestion{Type: "mcq", Prompt: "Q" + string(rune('A'+i)), Options: []string{"a", "b", "c", "d"}, CorrectIndices: []int{0}, Explanation: "e"}
	}
	b, _ := json.Marshal(app.GeneratedQuestionSet{Questions: qs})
	return string(b)
}

func input(t *testing.T) app.GenerateInput {
	t.Helper()
	doc, err := domain.NewDocument("notes.pdf", pdftest.Minimal("The cell membrane is selectively permeable"))
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := domain.NewQuestionSpec(5, nil)
	return app.GenerateInput{Document: doc, Spec: spec, Feedback: []string{"question 2: mcq must have exactly 4 options"}}
}

func newGen(t *testing.T, cfg openrouter.Config) *openrouter.Generator {
	t.Helper()
	if cfg.APIKey == "" {
		cfg.APIKey = "sk-test"
	}
	if cfg.Models == nil {
		cfg.Models = []string{"a/one:free", "b/two:free"}
	}
	g, err := openrouter.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestPDFNativeRequest(t *testing.T) {
	t.Parallel()
	srv, c := fakeOpenRouter(t, http.StatusOK, completion("b/two:free", questionsJSON(5)))
	g := newGen(t, openrouter.Config{BaseURL: srv.URL, Referer: "https://quiz.example", Title: "Quizette"})
	in := input(t)
	res, err := g.Generate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Drafts) != 5 || res.Drafts[0].Type != domain.MCQ {
		t.Fatalf("drafts = %+v", res.Drafts)
	}
	m := res.Metrics
	// The model that answered (here the fallback), and reasoning split out of completion tokens.
	if m.Model != "b/two:free" || m.InputTokens != 1200 || m.OutputTokens != 800 || m.ThinkingTokens != 300 || m.TotalTokens != 2300 || m.Strategy != domain.StrategyPDFNative {
		t.Errorf("metrics = %+v", m)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path != "/chat/completions" {
		t.Errorf("path = %s", c.path)
	}
	if got := c.header.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("Authorization = %q", got)
	}
	if c.header.Get("HTTP-Referer") != "https://quiz.example" || c.header.Get("X-OpenRouter-Title") != "Quizette" {
		t.Errorf("attribution headers = %v", c.header)
	}
	if models, _ := c.body["models"].([]any); len(models) != 2 || models[0] != "a/one:free" {
		t.Errorf("models = %v", c.body["models"])
	}
	rf, _ := c.body["response_format"].(map[string]any)
	js, _ := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["strict"] != true || js["schema"] == nil {
		t.Errorf("response_format = %v", rf)
	}
	if p, _ := c.body["provider"].(map[string]any); p["require_parameters"] != true {
		t.Errorf("provider = %v", c.body["provider"])
	}
	// The engine must be explicit: OpenRouter's default may be the paid mistral-ocr.
	plugins, _ := json.Marshal(c.body["plugins"])
	if string(plugins) != `[{"id":"file-parser","pdf":{"engine":"cloudflare-ai"}}]` {
		t.Errorf("plugins = %s", plugins)
	}
	msgs, _ := json.Marshal(c.body["messages"])
	if !strings.Contains(string(msgs), "EXACTLY 5 questions") || !strings.Contains(string(msgs), "must have exactly 4 options") {
		t.Errorf("system message missing spec/feedback: %.300s", msgs)
	}
	wantData := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(in.Document.Bytes())
	if !strings.Contains(string(msgs), `"filename":"notes.pdf"`) || !strings.Contains(string(msgs), wantData) {
		t.Errorf("pdf file part missing: %.400s", msgs)
	}
}

func TestTextExtractRequest(t *testing.T) {
	t.Parallel()
	srv, c := fakeOpenRouter(t, http.StatusOK, completion("a/one:free", questionsJSON(5)))
	g := newGen(t, openrouter.Config{BaseURL: srv.URL, Models: []string{"a/one:free"}, Strategy: domain.StrategyTextExtract, Extractor: ledongthuc.Extractor{}})
	res, err := g.Generate(context.Background(), input(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Metrics.ExtractedChars == 0 || res.Metrics.Strategy != domain.StrategyTextExtract {
		t.Errorf("metrics = %+v", res.Metrics)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.body["model"] != "a/one:free" || c.body["models"] != nil {
		t.Errorf("single model should use model, got model=%v models=%v", c.body["model"], c.body["models"])
	}
	if c.body["plugins"] != nil {
		t.Errorf("text mode should not use the PDF parser: %v", c.body["plugins"])
	}
	msgs, _ := json.Marshal(c.body["messages"])
	if strings.Contains(string(msgs), "application/pdf") || !strings.Contains(string(msgs), "selectively permeable") {
		t.Errorf("unexpected messages: %.300s", msgs)
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	errBody := func(code any, msg string) map[string]any {
		return map[string]any{"error": map[string]any{"code": code, "message": msg}}
	}
	tests := []struct {
		name            string
		status          int
		body            any
		wantUnavailable bool
		wantValidation  bool
	}{
		{"rate limited", 429, errBody(429, "rate limited"), true, false},
		{"no provider available", 503, errBody(503, "no provider"), true, false},
		{"model down", 502, errBody(502, "down"), true, false},
		{"bad request", 400, errBody(400, "bad schema"), false, false},
		{"bad key", 401, errBody(401, "no auth"), false, false},
		{"out of credits", 402, errBody(402, "credits"), false, false},
		{"error inside HTTP 200", 200, errBody(502, "provider failed"), true, false},
		{"choice finished with error", 200, map[string]any{"model": "a/one:free", "choices": []any{map[string]any{
			"finish_reason": "error", "message": map[string]any{"content": "{\"qu"}, "error": map[string]any{"code": 502, "message": "cut off"},
		}}}, true, false},
		{"no choices", 200, map[string]any{"model": "a/one:free", "choices": []any{}}, true, false},
		{"not JSON content", 200, completion("a/one:free", "Sure! Here are your questions"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv, _ := fakeOpenRouter(t, tt.status, tt.body)
			g := newGen(t, openrouter.Config{BaseURL: srv.URL})
			_, err := g.Generate(context.Background(), input(t))
			if err == nil {
				t.Fatal("want error")
			}
			if got := errors.Is(err, app.ErrLLMUnavailable); got != tt.wantUnavailable {
				t.Errorf("ErrLLMUnavailable = %v, want %v (err: %v)", got, tt.wantUnavailable, err)
			}
			var ve *domain.ValidationError
			if got := errors.As(err, &ve); got != tt.wantValidation {
				t.Errorf("ValidationError = %v, want %v (err: %v)", got, tt.wantValidation, err)
			}
		})
	}
}

func TestFencedJSONIsAccepted(t *testing.T) {
	t.Parallel()
	srv, _ := fakeOpenRouter(t, http.StatusOK, completion("a/one:free", "```json\n"+questionsJSON(5)+"\n```"))
	res, err := newGen(t, openrouter.Config{BaseURL: srv.URL}).Generate(context.Background(), input(t))
	if err != nil || len(res.Drafts) != 5 {
		t.Fatalf("drafts = %d, err = %v", len(res.Drafts), err)
	}
}

func TestUnreachableIsUnavailable(t *testing.T) {
	t.Parallel()
	srv, _ := fakeOpenRouter(t, http.StatusOK, nil)
	srv.Close()
	_, err := newGen(t, openrouter.Config{BaseURL: srv.URL}).Generate(context.Background(), input(t))
	if !errors.Is(err, app.ErrLLMUnavailable) {
		t.Errorf("want ErrLLMUnavailable, got %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()
	bad := []openrouter.Config{
		{Models: []string{"m"}},              // no key
		{APIKey: "k"},                        // no models
		{APIKey: "k", Models: []string{" "}}, // only blank models
		{APIKey: "k", Models: []string{"m"}, Strategy: domain.StrategyTextExtract}, // no extractor
		{APIKey: "k", Models: []string{"m"}, Strategy: "ocr"},
	}
	for i, cfg := range bad {
		if _, err := openrouter.New(cfg); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

// TestLiveOpenRouter hits the real API; skipped unless OPENROUTER_API_KEY is set.
// OPENROUTER_MODELS (comma-separated) picks the models.
func TestLiveOpenRouter(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("OPENROUTER_API_KEY not set")
	}
	models := strings.Split(os.Getenv("OPENROUTER_MODELS"), ",")
	if os.Getenv("OPENROUTER_MODELS") == "" {
		models = []string{"nvidia/nemotron-3.5-super-vl:free"}
	}
	for _, strategy := range []string{domain.StrategyPDFNative, domain.StrategyTextExtract} {
		t.Run(strategy, func(t *testing.T) {
			g, err := openrouter.New(openrouter.Config{APIKey: key, Models: models, Strategy: strategy, Extractor: ledongthuc.Extractor{}, Title: "Quizette tests"})
			if err != nil {
				t.Fatal(err)
			}
			doc, _ := domain.NewDocument("cells.pdf", pdftest.Minimal(
				"Cells are the basic unit of life. The nucleus stores DNA. Mitochondria produce ATP through cellular respiration.",
				"Plant cells have a cell wall made of cellulose and chloroplasts for photosynthesis. Animal cells lack a cell wall."))
			spec, _ := domain.NewQuestionSpec(5, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			res, err := g.Generate(ctx, app.GenerateInput{Document: doc, Spec: spec})
			t.Logf("metrics: %+v", res.Metrics)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := domain.NewQuestionSet("s", "c", spec, res.Drafts, func() domain.ID { return "x" }, time.Now()); err != nil {
				t.Errorf("validation: %v", err)
			}
		})
	}
}
