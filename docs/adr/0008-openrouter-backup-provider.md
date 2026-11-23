# 8. OpenRouter as a backup LLM provider

Status: accepted (2026-11-23). Extends [ADR 0002](0002-llm-behind-question-generator-port.md).

## Context
On 2026-11-23 Gemini's free tier was overloaded for hours. 14 of 16 benchmark calls failed ([ADR 0005](0005-generation-strategy.md)), and a real upload failed even after falling back across three Gemini models: two returned 503, and the last hung until `JOB_TIMEOUT`. Every model was on one provider's capacity, and a hung call used up the whole job budget.

[docs/research/llm-models.md](../research/llm-models.md) found OpenRouter to be the only free option that takes PDFs and supports strict JSON-schema output.

## Decision
- **New adapter.** `adapters/llm/openrouter` implements `QuestionGenerator` on OpenRouter's Chat Completions API:
  - Strict `json_schema` output with `provider.require_parameters`, so requests only go to providers that enforce the schema.
  - OpenRouter's `models` list handles fallback between its own models.
  - `pdf-native` sends the PDF with the file-parser plugin set to the free `cloudflare-ai` engine. Left unset, OpenRouter may pick the paid `mistral-ocr`. `text-extract` sends locally extracted text.
  - OpenRouter can report failures inside an HTTP 200 response (a top-level `error`, or `finish_reason: "error"`); these are treated as errors.
- **Provider chain.** `adapters/llm/fallback` chains providers. With `LLM_PROVIDER=gemini` and `OPENROUTER_API_KEY` set, the chain is Gemini then OpenRouter. `LLM_PROVIDER=openrouter` uses OpenRouter alone.
- **When to move on.** Adapters wrap transient failures (408, 429, 5xx, network errors) in `app.ErrLLMUnavailable`. The chain moves on only for those, or when a provider exceeds `LLM_ATTEMPT_TIMEOUT` (default 90 s). The last provider gets whatever remains of `JOB_TIMEOUT`. Validation errors don't move on; the app's retry-with-feedback handles them.
- **User message.** When every provider is unavailable, the job fails with "the AI models are busy right now".

## Consequences
- Free OpenRouter models are capped at 50 requests a day (1,000 after buying 10 credits), so OpenRouter is the backup, not the primary.
- **Only `:free` models should be listed.** OpenRouter bills a fallback to a paid model at the paid price.
- Free models differ in quality and change often. `OPENROUTER_MODELS` makes them configurable. The default is `nvidia/nemotron-3.5-super-vl:free` alone, from a live test on 2026-11-23 (2-page sample PDF, 5 questions):

  | Model | pdf-native | text-extract | Kept? |
  |---|---|---|---|
  | `nvidia/nemotron-3.5-super-vl:free` | ✅ valid, 38 s, 4,748 tokens | ✅ valid, 44 s, 4,357 tokens | Yes |
  | `baidu/ernie-5.5-flash-preview:free` | ⚠️ 1 of 5 questions valid (5-option MCQs, wrong True/False options) | ❌ 429 | No |
  | `inclusionai/ling-3.2-flash:free` | ❌ 429 | ❌ 429 | No (untested) |

  Nemotron spends most of its tokens thinking (3.3–3.6k of ~4.5k), so expect it to be slow on full lectures.
- OpenRouter's `cloudflare-ai` parser converts the PDF to text, so scanned PDFs still need Gemini.
- With the defaults (90 s per attempt, 3 min per job), Gemini gets 90 s and OpenRouter gets the remaining ~90 s. Large lectures took 53–57 s on Gemini; raise `JOB_TIMEOUT` if successful calls start timing out.
