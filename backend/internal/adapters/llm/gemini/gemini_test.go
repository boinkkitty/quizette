package gemini_test

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

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/gemini"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/ledongthuc"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type captured struct {
	mu   sync.Mutex
	path string
	body map[string]any
}

func fakeGemini(t *testing.T, responseText string) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.path = r.URL.Path
		_ = json.Unmarshal(raw, &c.body)
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{map[string]any{"content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": responseText}}}}},
			"usageMetadata": map[string]any{
				"promptTokenCount": 1200, "candidatesTokenCount": 800, "thoughtsTokenCount": 300, "totalTokenCount": 2300,
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, c
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

func dig(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func TestPDFNativeSendsSchemaAndPDF(t *testing.T) {
	t.Parallel()
	srv, c := fakeGemini(t, questionsJSON(5))
	g, err := gemini.New(context.Background(), gemini.Config{APIKey: "k", BaseURL: srv.URL, Strategy: domain.StrategyPDFNative})
	if err != nil {
		t.Fatal(err)
	}
	in := input(t)
	res, err := g.Generate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Drafts) != 5 || res.Drafts[0].Type != domain.MCQ {
		t.Fatalf("drafts = %+v", res.Drafts)
	}
	m := res.Metrics
	if m.InputTokens != 1200 || m.OutputTokens != 800 || m.ThinkingTokens != 300 || m.TotalTokens != 2300 || m.Strategy != domain.StrategyPDFNative || m.Model != gemini.DefaultModel {
		t.Errorf("metrics = %+v", m)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !strings.Contains(c.path, "gemini-3.8-flash:generateContent") {
		t.Errorf("path = %s", c.path)
	}
	if got := dig(c.body, "generationConfig", "responseMimeType"); got != "application/json" {
		t.Errorf("responseMimeType = %v", got)
	}
	schema, _ := dig(c.body, "generationConfig", "responseJsonSchema").(map[string]any)
	if schema == nil || dig(schema, "properties", "questions", "type") != "array" {
		t.Errorf("schema not sent: %v", dig(c.body, "generationConfig"))
	}
	// Gemini rejects a minItems+maxItems range on the outer array ("schema produces a constraint
	// that has too many states for serving"), so the upper bound lives in the prompt and the domain.
	if v := dig(schema, "properties", "questions", "maxItems"); v != nil {
		t.Errorf("questions.maxItems = %v; Gemini can't serve a bounded outer array", v)
	}
	sys, _ := json.Marshal(dig(c.body, "systemInstruction"))
	if !strings.Contains(string(sys), "EXACTLY 5 questions") || !strings.Contains(string(sys), "must have exactly 4 options") {
		t.Errorf("system instruction missing spec/feedback: %s", sys)
	}
	contents, _ := json.Marshal(c.body["contents"])
	if !strings.Contains(string(contents), `"mimeType":"application/pdf"`) {
		t.Errorf("pdf part missing: %.300s", contents)
	}
	if !strings.Contains(string(contents), base64.StdEncoding.EncodeToString(in.Document.Bytes())[:20]) {
		t.Error("pdf bytes not inlined")
	}
}

func TestTextExtractSendsText(t *testing.T) {
	t.Parallel()
	srv, c := fakeGemini(t, questionsJSON(5))
	g, err := gemini.New(context.Background(), gemini.Config{APIKey: "k", BaseURL: srv.URL, Strategy: domain.StrategyTextExtract, Extractor: ledongthuc.Extractor{}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := g.Generate(context.Background(), input(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Metrics.ExtractedChars == 0 || res.Metrics.Strategy != domain.StrategyTextExtract {
		t.Errorf("metrics = %+v", res.Metrics)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	contents, _ := json.Marshal(c.body["contents"])
	if strings.Contains(string(contents), "application/pdf") || !strings.Contains(string(contents), "selectively permeable") {
		t.Errorf("unexpected contents: %.300s", contents)
	}
}

func TestInvalidJSONIsValidationError(t *testing.T) {
	t.Parallel()
	srv, _ := fakeGemini(t, "not json")
	g, _ := gemini.New(context.Background(), gemini.Config{APIKey: "k", BaseURL: srv.URL})
	res, err := g.Generate(context.Background(), input(t))
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if res.Metrics.TotalTokens != 2300 {
		t.Error("metrics should be returned on error")
	}
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, err := gemini.New(ctx, gemini.Config{}); err == nil {
		t.Error("missing key: want error")
	}
	if _, err := gemini.New(ctx, gemini.Config{APIKey: "k", Strategy: domain.StrategyTextExtract}); err == nil {
		t.Error("text mode without extractor: want error")
	}
	if _, err := gemini.New(ctx, gemini.Config{APIKey: "k", Strategy: "ocr"}); err == nil {
		t.Error("unknown strategy: want error")
	}
}

// TestLiveGemini hits the real API; skipped unless GEMINI_API_KEY is set.
func TestLiveGemini(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("GEMINI_API_KEY not set")
	}
	// GEMINI_MODEL picks the model, as it does for the API; empty means gemini.DefaultModel.
	g, err := gemini.New(context.Background(), gemini.Config{APIKey: key, Model: os.Getenv("GEMINI_MODEL")})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := domain.NewDocument("cells.pdf", pdftest.Minimal(
		"Cells are the basic unit of life. The nucleus stores DNA. Mitochondria produce ATP through cellular respiration.",
		"Plant cells have a cell wall made of cellulose and chloroplasts for photosynthesis. Animal cells lack a cell wall."))
	spec, _ := domain.NewQuestionSpec(5, nil)
	res, err := g.Generate(context.Background(), app.GenerateInput{Document: doc, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := domain.NewQuestionSet("s", "c", spec, res.Drafts, func() domain.ID { return "x" }, time.Now()); err != nil {
		t.Logf("validation: %v", err)
	}
	t.Logf("metrics: %+v", res.Metrics)
}
