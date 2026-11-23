// Package openrouter implements app.QuestionGenerator with OpenRouter's
// OpenAI-compatible Chat Completions API, which fronts many models, including
// free ":free" variants.
//
// Output is constrained with a strict JSON schema, and requests are routed
// only to providers that support it. Like the gemini adapter, it supports two
// input strategies: sending the PDF (parsed by OpenRouter's file-parser plugin)
// or sending text extracted by an app.TextExtractor.
package openrouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// DefaultBaseURL is OpenRouter's API root.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// DefaultPDFEngine is the free PDF parser. Without an explicit engine,
// OpenRouter may fall back to the paid mistral-ocr engine.
const DefaultPDFEngine = "cloudflare-ai"

// maxTextChars caps extracted text sent in text mode.
const maxTextChars = 1_000_000

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 4 << 20

// Config configures the generator.
type Config struct {
	APIKey string
	// Models are tried in order by OpenRouter itself (its "models" routing)
	// when a model errors, is rate limited or is down. At least one is required.
	// Only list ":free" models to stay free: a fallback to a paid model is billed.
	Models []string
	// Strategy is domain.StrategyPDFNative or domain.StrategyTextExtract.
	Strategy string
	// Extractor is required for the text-extract strategy.
	Extractor app.TextExtractor
	// PDFEngine is the file-parser engine for the pdf-native strategy
	// (default DefaultPDFEngine).
	PDFEngine string
	// Referer and Title are optional attribution headers shown on openrouter.ai.
	Referer, Title string
	// BaseURL and HTTPClient are for tests.
	BaseURL    string
	HTTPClient *http.Client
}

// Generator calls OpenRouter with schema-constrained JSON output. It is safe
// for concurrent use.
type Generator struct {
	cfg Config
}

// New creates a Generator.
func New(cfg Config) (*Generator, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("openrouter: API key is required")
	}
	models := cfg.Models[:0:0]
	for _, m := range cfg.Models {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	if len(models) == 0 {
		return nil, errors.New("openrouter: at least one model is required")
	}
	cfg.Models = models
	switch cfg.Strategy {
	case "":
		cfg.Strategy = domain.StrategyPDFNative
	case domain.StrategyPDFNative:
	case domain.StrategyTextExtract:
		if cfg.Extractor == nil {
			return nil, errors.New("openrouter: text-extract strategy requires an extractor")
		}
	default:
		return nil, fmt.Errorf("openrouter: unknown strategy %q", cfg.Strategy)
	}
	if cfg.PDFEngine == "" {
		cfg.PDFEngine = DefaultPDFEngine
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	return &Generator{cfg: cfg}, nil
}

type request struct {
	Model          string         `json:"model,omitempty"`
	Models         []string       `json:"models,omitempty"`
	Messages       []message      `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
	Provider       providerPrefs  `json:"provider"`
	Plugins        []plugin       `json:"plugins,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentPart
}

type contentPart struct {
	Type string    `json:"type"`
	Text string    `json:"text,omitempty"`
	File *filePart `json:"file,omitempty"`
}

type filePart struct {
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type providerPrefs struct {
	// RequireParameters routes only to providers that support every
	// request parameter, here the JSON schema.
	RequireParameters bool `json:"require_parameters"`
}

type plugin struct {
	ID  string    `json:"id"`
	PDF pdfPlugin `json:"pdf"`
}

type pdfPlugin struct {
	Engine string `json:"engine"`
}

type response struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
		FinishReason string    `json:"finish_reason"`
		Error        *apiError `json:"error"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error *apiError `json:"error"`
}

// apiError is OpenRouter's error object. Code is usually an HTTP status number.
type apiError struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
}

func (e *apiError) status() int {
	n, _ := strconv.Atoi(strings.Trim(string(e.Code), `"`))
	return n
}

// Generate implements app.QuestionGenerator. Metrics are returned even on
// error. Metrics.Model is the model that answered, as reported by OpenRouter.
func (g *Generator) Generate(ctx context.Context, in app.GenerateInput) (app.GenerationResult, error) {
	start := time.Now()
	res := app.GenerationResult{Metrics: domain.GenerationMetrics{Strategy: g.cfg.Strategy, Model: g.cfg.Models[0]}}

	body := request{
		Messages: []message{{Role: "system", Content: app.GenerationInstructions(in.Spec, in.Feedback)}},
		ResponseFormat: responseFormat{Type: "json_schema", JSONSchema: jsonSchema{
			Name: "question_set", Strict: true, Schema: app.QuestionSetSchema(),
		}},
		Provider: providerPrefs{RequireParameters: true},
	}
	if len(g.cfg.Models) == 1 {
		body.Model = g.cfg.Models[0]
	} else {
		body.Models = g.cfg.Models
	}

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
		body.Messages = append(body.Messages, message{
			Role:    "user",
			Content: "<document>\n" + text + "\n</document>\n\nGenerate the questions from the document above.",
		})
	default:
		body.Messages = append(body.Messages, message{Role: "user", Content: []contentPart{
			{Type: "text", Text: "Generate the questions from the attached PDF."},
			{Type: "file", File: &filePart{
				Filename: in.Document.Filename(),
				FileData: "data:" + domain.PDFMediaType + ";base64," + base64.StdEncoding.EncodeToString(in.Document.Bytes()),
			}},
		}})
		body.Plugins = []plugin{{ID: "file-parser", PDF: pdfPlugin{Engine: g.cfg.PDFEngine}}}
	}

	t1 := time.Now()
	resp, err := g.call(ctx, body)
	res.Metrics.LLMMS = time.Since(t1).Milliseconds()
	if resp != nil {
		if resp.Model != "" {
			res.Metrics.Model = resp.Model
		}
		if u := resp.Usage; u != nil {
			res.Metrics.InputTokens = u.PromptTokens
			res.Metrics.TotalTokens = u.TotalTokens
			res.Metrics.OutputTokens = u.CompletionTokens
			if u.CompletionTokensDetails != nil {
				// completion_tokens includes reasoning tokens (OpenAI convention);
				// split them so metrics match the gemini adapter's.
				res.Metrics.ThinkingTokens = u.CompletionTokensDetails.ReasoningTokens
				res.Metrics.OutputTokens -= res.Metrics.ThinkingTokens
			}
		}
	}
	res.Metrics.TotalMS = time.Since(start).Milliseconds()
	if err != nil {
		return res, fmt.Errorf("openrouter generate (%s): %w", res.Metrics.Model, err)
	}

	var out app.GeneratedQuestionSet
	if err := json.Unmarshal([]byte(stripFences(*resp.Choices[0].Message.Content)), &out); err != nil {
		// Treated as a validation failure so the app retries with feedback.
		return res, domain.Invalid("response was not valid JSON matching the schema")
	}
	res.Drafts = out.ToDrafts()
	return res, nil
}

// call sends one Chat Completions request. It returns the decoded response,
// when there is one, alongside any error. Transient failures wrap
// app.ErrLLMUnavailable.
func (g *Generator) call(ctx context.Context, body request) (*response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if g.cfg.Referer != "" {
		req.Header.Set("HTTP-Referer", g.cfg.Referer)
	}
	if g.cfg.Title != "" {
		req.Header.Set("X-OpenRouter-Title", g.cfg.Title)
	}

	httpResp, err := g.cfg.HTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("send request: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%w: send request: %w", app.ErrLLMUnavailable, err)
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes))
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("read response: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%w: read response: %w", app.ErrLLMUnavailable, err)
	}

	var resp response
	decodeErr := json.Unmarshal(data, &resp)
	if httpResp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(data))
		if decodeErr == nil && resp.Error != nil {
			msg = resp.Error.Message
		}
		return nil, statusError(httpResp.StatusCode, msg)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("%w: decode response: %w", app.ErrLLMUnavailable, decodeErr)
	}
	// OpenRouter reports failures during generation with HTTP 200: either a
	// top-level error, or a choice that finished with "error".
	if resp.Error != nil {
		return &resp, statusError(resp.Error.status(), resp.Error.Message)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == nil {
		return &resp, fmt.Errorf("%w: response has no content", app.ErrLLMUnavailable)
	}
	if c := resp.Choices[0]; c.FinishReason == "error" {
		msg := "generation failed"
		if c.Error != nil {
			msg = c.Error.Message
		}
		return &resp, fmt.Errorf("%w: %s", app.ErrLLMUnavailable, msg)
	}
	return &resp, nil
}

// statusError builds the error for a failed request. Timeouts, rate limits and
// server errors are transient; anything else (bad request, auth, credits,
// moderation) is not. An unknown status (0) from a mid-generation failure is
// treated as transient.
func statusError(status int, msg string) error {
	err := fmt.Errorf("HTTP %d: %.300s", status, msg)
	if status == 0 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500 {
		return fmt.Errorf("%w: %w", app.ErrLLMUnavailable, err)
	}
	return err
}

// stripFences removes a Markdown code fence some models wrap JSON in.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	return strings.TrimSpace(strings.TrimSuffix(s, "```"))
}
