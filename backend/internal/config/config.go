// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Config is the full runtime configuration.
type Config struct {
	Env         string // development | production
	Port        string
	DatabaseURL string
	PublicURL   string // browser-facing origin (Worker / Vite dev server)
	LogLevel    slog.Level
	Version     string

	LLMProvider          string // gemini | openrouter | stub
	GeminiAPIKey         string
	GeminiModel          string
	GeminiFallbackModels []string
	GenerationStrategy   string
	GeminiThinkingBudget *int32
	StubLLMDelay         time.Duration

	// OpenRouter is the only provider when LLMProvider is "openrouter", and the
	// backup provider after Gemini when LLMProvider is "gemini" and the key is set.
	OpenRouterAPIKey    string
	OpenRouterModels    []string
	OpenRouterPDFEngine string
	// LLMAttemptTimeout bounds each provider except the last, so a slow or hung
	// provider leaves time for the next one within JobTimeout.
	LLMAttemptTimeout time.Duration

	JobWorkers   int
	JobQueueSize int
	JobTimeout   time.Duration

	GoogleClientID, GoogleClientSecret string
	GitHubClientID, GitHubClientSecret string
}

// Production reports whether this is a production deployment.
func (c Config) Production() bool { return c.Env == "production" }

// Load reads and validates configuration. getenv is os.Getenv in production.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	str := func(k, def string) string {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	var errs []error
	num := func(k string, def int) int {
		v := getenv(k)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
		return n
	}
	dur := func(k string, def time.Duration) time.Duration {
		v := getenv(k)
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
		return d
	}

	c := Config{
		Env:                 str("APP_ENV", "development"),
		Port:                str("PORT", ""),
		DatabaseURL:         str("DATABASE_URL", ""),
		PublicURL:           strings.TrimRight(str("PUBLIC_URL", ""), "/"),
		Version:             str("RENDER_GIT_COMMIT", str("VERSION", "dev")),
		LLMProvider:         str("LLM_PROVIDER", "gemini"),
		GeminiAPIKey:        str("GEMINI_API_KEY", ""),
		GeminiModel:         str("GEMINI_MODEL", "gemini-3.6-flash"),
		GenerationStrategy:  str("GENERATION_STRATEGY", domain.StrategyPDFNative),
		StubLLMDelay:        dur("STUB_LLM_DELAY", 0),
		OpenRouterAPIKey:    str("OPENROUTER_API_KEY", ""),
		OpenRouterPDFEngine: str("OPENROUTER_PDF_ENGINE", "cloudflare-ai"),
		LLMAttemptTimeout:   dur("LLM_ATTEMPT_TIMEOUT", 90*time.Second),
		JobWorkers:          num("JOB_WORKERS", 2),
		JobQueueSize:        num("JOB_QUEUE_SIZE", 20),
		JobTimeout:          dur("JOB_TIMEOUT", 3*time.Minute),
		GoogleClientID:      str("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  str("GOOGLE_CLIENT_SECRET", ""),
		GitHubClientID:      str("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  str("GITHUB_CLIENT_SECRET", ""),
	}
	// Both defaults passed the pipeline test in docs/research/llm-models.md.
	// "none" turns fallback off.
	c.GeminiFallbackModels = list(str("GEMINI_FALLBACK_MODELS", "gemini-3.5-flash,gemini-3.1-flash-lite"))
	// Free models that supported structured outputs on 2026-11-23. OpenRouter's
	// free catalogue changes often; check https://openrouter.ai/models?q=free.
	c.OpenRouterModels = list(str("OPENROUTER_MODELS", "nvidia/nemotron-3.5-super-vl:free,baidu/ernie-5.5-flash-preview:free,inclusionai/ling-3.2-flash:free"))
	if v := getenv("GEMINI_THINKING_BUDGET"); v != "" {
		n := int32(num("GEMINI_THINKING_BUDGET", 0)) //nolint:gosec // small config value
		c.GeminiThinkingBudget = &n
	}
	if err := c.LogLevel.UnmarshalText([]byte(str("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	// Deployment-specific settings have no defaults so a missing value fails at boot.
	for _, req := range []struct{ key, val string }{
		{"PORT", c.Port}, {"DATABASE_URL", c.DatabaseURL}, {"PUBLIC_URL", c.PublicURL},
	} {
		if req.val == "" {
			errs = append(errs, fmt.Errorf("%s is required", req.key))
		}
	}
	switch c.LLMProvider {
	case "gemini":
		if c.GeminiAPIKey == "" {
			errs = append(errs, errors.New("GEMINI_API_KEY is required when LLM_PROVIDER=gemini (use LLM_PROVIDER=stub for local dev)"))
		}
	case "openrouter":
		if c.OpenRouterAPIKey == "" {
			errs = append(errs, errors.New("OPENROUTER_API_KEY is required when LLM_PROVIDER=openrouter"))
		}
	case "stub":
		if c.Production() {
			errs = append(errs, errors.New("LLM_PROVIDER=stub is not allowed in production"))
		}
	default:
		errs = append(errs, fmt.Errorf("LLM_PROVIDER must be gemini, openrouter or stub, got %q", c.LLMProvider))
	}
	switch c.GenerationStrategy {
	case domain.StrategyPDFNative, domain.StrategyTextExtract:
	default:
		errs = append(errs, fmt.Errorf("GENERATION_STRATEGY must be %s or %s", domain.StrategyPDFNative, domain.StrategyTextExtract))
	}
	if c.Env != "development" && c.Env != "production" && c.Env != "test" {
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test or production, got %q", c.Env))
	}
	if c.OpenRouterAPIKey != "" && len(c.OpenRouterModels) == 0 {
		errs = append(errs, errors.New("OPENROUTER_MODELS must list at least one model when OPENROUTER_API_KEY is set"))
	}
	return c, errors.Join(errs...)
}

// list splits a comma-separated setting, dropping blanks. "none" yields nil.
func list(v string) []string {
	if v == "none" {
		return nil
	}
	var out []string
	for _, m := range strings.Split(v, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}
