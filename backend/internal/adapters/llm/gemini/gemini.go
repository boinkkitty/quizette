// Package gemini implements app.QuestionGenerator with Google's Gemini API.
// It supports two input strategies: sending the PDF bytes natively, or
// sending text extracted by an app.TextExtractor.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// DefaultModel is the free-tier model used unless configured otherwise.
const DefaultModel = "gemini-2.5-flash"

// maxTextChars caps extracted text sent in text mode.
const maxTextChars = 1_000_000

// Config configures the generator.
type Config struct {
	APIKey   string
	Model    string
	Strategy string // domain.StrategyPDFNative or domain.StrategyTextExtract
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

// Generate implements app.QuestionGenerator. Metrics are returned even on error.
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

	t1 := time.Now()
	resp, err := g.client.Models.GenerateContent(ctx, g.cfg.Model, []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}, config)
	res.Metrics.LLMMS = time.Since(t1).Milliseconds()
	if resp != nil && resp.UsageMetadata != nil {
		u := resp.UsageMetadata
		res.Metrics.InputTokens = int(u.PromptTokenCount)
		res.Metrics.OutputTokens = int(u.CandidatesTokenCount)
		res.Metrics.ThinkingTokens = int(u.ThoughtsTokenCount)
		res.Metrics.TotalTokens = int(u.TotalTokenCount)
	}
	res.Metrics.TotalMS = time.Since(start).Milliseconds()
	if err != nil {
		return res, fmt.Errorf("gemini generate: %w", err)
	}

	var out app.GeneratedQuestionSet
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		// Treated as a validation failure so the app retries with feedback.
		return res, domain.Invalid("response was not valid JSON matching the schema")
	}
	res.Drafts = out.ToDrafts()
	return res, nil
}
