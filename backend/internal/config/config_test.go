package config_test

import (
	"slices"
	"testing"

	"github.com/boinkkitty/quizette/backend/internal/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad(t *testing.T) {
	t.Parallel()
	c, err := config.Load(env(map[string]string{"PORT": "9000", "DATABASE_URL": "postgres://x", "LLM_PROVIDER": "stub", "PUBLIC_URL": "https://app.example/"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9000" || c.PublicURL != "https://app.example" || c.GenerationStrategy != "pdf-native" {
		t.Errorf("defaults: %+v", c)
	}

	if want := []string{"gemini-3.5-flash", "gemini-3.1-flash-lite"}; !slices.Equal(c.GeminiFallbackModels, want) {
		t.Errorf("fallback models = %v, want %v", c.GeminiFallbackModels, want)
	}
	for v, want := range map[string][]string{"none": nil, "a, ,b": {"a", "b"}} {
		c, err := config.Load(env(map[string]string{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "GEMINI_FALLBACK_MODELS": v}))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(c.GeminiFallbackModels, want) {
			t.Errorf("GEMINI_FALLBACK_MODELS=%q: got %v, want %v", v, c.GeminiFallbackModels, want)
		}
	}

	if c.LLMAttemptTimeout.String() != "1m30s" || c.OpenRouterPDFEngine != "cloudflare-ai" || len(c.OpenRouterModels) == 0 || c.OpenRouterAPIKey != "" {
		t.Errorf("openrouter defaults: %+v", c)
	}
	if _, err := config.Load(env(map[string]string{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "openrouter", "OPENROUTER_API_KEY": "k"})); err != nil {
		t.Errorf("openrouter-only config (no Gemini key): %v", err)
	}

	bad := []map[string]string{
		{}, // nothing set
		{"DATABASE_URL": "x", "LLM_PROVIDER": "stub", "PUBLIC_URL": "https://a"}, // no PORT
		{"PORT": "1", "LLM_PROVIDER": "stub", "PUBLIC_URL": "https://a"},         // no DATABASE_URL
		{"PORT": "1", "DATABASE_URL": "x", "LLM_PROVIDER": "stub"},               // no PUBLIC_URL
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a"},            // gemini without key
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "APP_ENV": "production"},
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "GENERATION_STRATEGY": "ocr"},
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "JOB_TIMEOUT": "soon"},
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "openrouter"},                                                   // no OpenRouter key
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "OPENROUTER_API_KEY": "k", "OPENROUTER_MODELS": "none"}, // key but no models
		{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "LLM_PROVIDER": "stub", "LLM_ATTEMPT_TIMEOUT": "soon"},
	}
	if _, err := config.Load(env(map[string]string{"PORT": "1", "DATABASE_URL": "x", "PUBLIC_URL": "https://a", "GEMINI_API_KEY": "k", "APP_ENV": "production"})); err != nil {
		t.Errorf("minimal production config: %v", err)
	}

	for i, m := range bad {
		if _, err := config.Load(env(m)); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}
