# 2. LLMs behind a QuestionGenerator port with a provider-neutral JSON Schema

Status: accepted (2026-11-20)

## Context
We start on Gemini's free tier but want to switch providers (Claude, OpenAI, Ollama) without touching business rules, and LLM output can't be trusted.

## Decision
- `app.QuestionGenerator.Generate(ctx, GenerateInput) (GenerationResult, error)` returns drafts plus metrics (tokens, timings, strategy, model).
- One JSON Schema (`internal/app/question_set.schema.json`) and one prompt (`internal/app/prompt.go`) are shared by all adapters; each adapter maps the schema onto its provider's structured-output feature (Gemini: `responseMimeType: application/json` + `responseJsonSchema`).
- The schema guarantees shape; `domain.NewQuestionSet` guarantees meaning (anti-corruption layer). On too few valid questions the job retries once with the violations appended to the prompt.
- Provider selected by `LLM_PROVIDER` (`gemini` | `stub`).

## Consequences
Adding a provider = one adapter package + a config switch. The deterministic `stub` adapter powers tests, E2E and offline development.
