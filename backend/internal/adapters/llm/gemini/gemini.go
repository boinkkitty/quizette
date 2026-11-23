// Package gemini implements app.QuestionGenerator with Google's Gemini API.
// It supports two input strategies: sending the PDF bytes natively, or
// sending text extracted by an app.TextExtractor.
//
// When the primary model fails with a transient error (rate limit, overload
// or server error), the generator retries the same request on each fallback
// model in order.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

// DefaultModel is the free-tier model used unless configured otherwise.
const DefaultModel = "gemini-3.6-flash"

// maxTextChars caps extracted text sent in text mode.
const maxTextChars = 1_000_000

// Config configures the generator.
type Config struct {
	APIKey string
	Model  string
	// FallbackModels are tried in order, after Model, when a call fails with a
	// transient error (HTTP 408, 429 or 5xx). Empty means no fallback.
	FallbackModels []string
	Strategy       string // domain.StrategyPDFNative or domain.StrategyTextExtract
	// Extractor is required for the text-extract strategy.
	Extractor app.TextExtractor
	// ThinkingBudget limits reasoning tokens (nil = model default, 0 = off).
	ThinkingBudget *int32
	// BaseURL and HTTPClient are for tests.
	BaseURL    string
	HTTPClient *http.Client
}

// Generator calls Gemini with schema-constrained JSON output.
type Generator struct {
	client *genai.Client
	cfg    Config
}

// New creates a Generator.
func New(ctx context.Context, cfg Config) (*Generator, error) {
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	switch cfg.Strategy {
	case "":
		cfg.Strategy = domain.StrategyPDFNative
	case domain.StrategyPDFNative:
	case domain.StrategyTextExtract:
		if cfg.Extractor == nil {
			return nil, errors.New("gemini: text-extract strategy requires an extractor")
		}
	default:
		return nil, fmt.Errorf("gemini: unknown strategy %q", cfg.Strategy)
	}
	if cfg.APIKey == "" {
		return nil, errors.New("gemini: API key is required")
	}
	cc := &genai.ClientConfig{APIKey: cfg.APIKey, Backend: genai.BackendGeminiAPI, HTTPClient: cfg.HTTPClient}
	if cfg.BaseURL != "" {
		cc.HTTPOptions = genai.HTTPOptions{BaseURL: cfg.BaseURL}
	}
	client, err := genai.NewClient(ctx, cc)
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}
	return &Generator{client: client, cfg: cfg}, nil
}

// Strategy reports the configured input strategy.
func (g *Generator) Strategy() string { return g.cfg.Strategy }

// Models returns the primary model followed by the fallback models, in the
// order they are tried.
func (g *Generator) Models() []string {
	models := []string{g.cfg.Model}
	for _, m := range g.cfg.FallbackModels {
		if m != "" && !slices.Contains(models, m) {
			models = append(models, m)
		}
	}
	return models
}

// Generate implements app.QuestionGenerator. Metrics are returned even on
// error; Metrics.Model is the model that produced the final response, and
// LLMMS and token counts describe that call only.
func (g *Generator) Generate(ctx context.Context, in app.GenerateInput) (app.GenerationResult, error) {
	start := time.Now()
	res := app.GenerationResult{Metrics: domain.GenerationMetrics{Strategy: g.cfg.Strategy, Model: g.cfg.Model}}

	var parts []*genai.Part
	switch g.cfg.Strategy {
	case domain.StrategyTextExtract:
		t0 := time.Now()
		text, err := g.cfg.Extractor.Extract(ctx, in.Document)
		res.Metrics.ExtractMS = time.Since(t0).Milliseconds()
		if err != nil {
			return res, fmt.Errorf("extract text: %w", err)
		}
		text = strings.TrimSpace(text)
		if len(text) > maxTextChars {
			text = text[:maxTextChars]
		}
		res.Metrics.ExtractedChars = len(text)
		if text == "" {
			return res, app.ErrUnreadableDocument
		}
		parts = []*genai.Part{genai.NewPartFromText("<document>\n" + text + "\n</document>\n\nGenerate the questions from the document above.")}
	default:
		parts = []*genai.Part{
			genai.NewPartFromBytes(in.Document.Bytes(), domain.PDFMediaType),
			genai.NewPartFromText("Generate the questions from the attached PDF."),
		}
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(app.GenerationInstructions(in.Spec, in.Feedback), genai.RoleUser),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: app.QuestionSetSchema(),
	}
	if g.cfg.ThinkingBudget != nil {
		config.ThinkingConfig = &genai.ThinkingConfig{ThinkingBudget: g.cfg.ThinkingBudget}
	}

	contents := []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}
	var resp *genai.GenerateContentResponse
	var err error
	models := g.Models()
	for i, model := range models {
		res.Metrics.Model = model
		t1 := time.Now()
		resp, err = g.client.Models.GenerateContent(ctx, model, contents, config)
		res.Metrics.LLMMS = time.Since(t1).Milliseconds()
		if err == nil || !transient(err) || ctx.Err() != nil || i == len(models)-1 {
			break
		}
		logctx.From(ctx, slog.Default()).WarnContext(ctx, "llm.fallback",
			slog.String("model", model), slog.String("next_model", models[i+1]), slog.Any("error", err))
	}
	if resp != nil && resp.UsageMetadata != nil {
		u := resp.UsageMetadata
		res.Metrics.InputTokens = int(u.PromptTokenCount)
		res.Metrics.OutputTokens = int(u.CandidatesTokenCount)
		res.Metrics.ThinkingTokens = int(u.ThoughtsTokenCount)
		res.Metrics.TotalTokens = int(u.TotalTokenCount)
	}
	res.Metrics.TotalMS = time.Since(start).Milliseconds()
	if err != nil {
		return res, fmt.Errorf("gemini generate (%s): %w", res.Metrics.Model, err)
	}

	var out app.GeneratedQuestionSet
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		// Treated as a validation failure so the app retries with feedback.
		return res, domain.Invalid("response was not valid JSON matching the schema")
	}
	res.Drafts = out.ToDrafts()
	return res, nil
}

// transient reports whether err is worth retrying on another model: a timeout,
// a rate limit or quota error, or a server-side failure such as an overload.
func transient(err error) bool {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == http.StatusRequestTimeout || apiErr.Code == http.StatusTooManyRequests || apiErr.Code >= 500
}
