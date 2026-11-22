# Free LLM models for question generation

Status: **research note**, not a decision. Tested 2026-11-22 and 2026-11-23. Free tiers, model names and availability change often, so re-check before relying on any number here.

Quizette sends a lecture PDF (or its extracted text) to an LLM and needs 5–20 questions back as JSON matching [`question_set.schema.json`](../../backend/internal/app/question_set.schema.json). Today only Gemini is wired up, behind the `QuestionGenerator` port ([ADR 0002](../adr/0002-llm-behind-question-generator-port.md)).

## Recommendation

1. **Use `gemini-3.6-flash`.** It passed every real test, takes the PDF directly, and is the newest Flash that was reliably available. This is the value in `backend/.env.local`.
2. **Fallback: `gemini-3.5-flash`.** It also passed every run.
3. **For speed or quota: `gemini-3.1-flash-lite`.** It was the fastest at about 3.4 s, has no thinking tokens (about 2.1k tokens per call against 3.2k), and its output was valid. Question quality hasn't been compared yet.
4. **Don't depend on `gemini-3.8-flash` (the code default) or `gemini-4-hydrogen` yet.** Both were "overloaded" (HTTP 503) most of the time. The app has no retry or fallback, so a 503 fails the upload.
5. **Other providers are a later option, not a replacement.** None gives a permanently free tier with native PDF input and strict JSON schema output together (see below).

## Gemini: what works

### How it was tested

- **2026-11-22, availability probe:** one tiny request ("Reply with ok") to every text model the key lists.
- **2026-11-23, pipeline test:** `TestLiveGemini` (`backend/internal/adapters/llm/gemini`), run twice per model. It sends a small 2-page PDF through our real adapter (`pdf-native`), with our schema and prompt, asking for 5 questions. Then it validates the result with the domain rules (`NewQuestionSet`). Command:
  ```sh
  cd backend && set -a; . ./.env.local; set +a
  GEMINI_MODEL=<model> go test ./internal/adapters/llm/gemini -run TestLiveGemini -count=1 -v
  ```
- Both runs used `pdf-native`, because the live test doesn't wire up text extraction. Comparing `pdf-native` with `text-extract` is the job of `make bench` (see [ADR 0005](../adr/0005-generation-strategy.md)).
- Free tier, one API key. Token counts are from Gemini's usage metadata.

### Results

| Model | Free tier | Pipeline run 1 | Pipeline run 2 | Tokens (in / out / thinking) | Latency | Verdict |
|---|---|---|---|---|---|---|
| **gemini-3.6-flash** | Yes | ✅ valid | ✅ valid | 1,521 / 623 / 1,105 | 7.1–13.9 s | **Use this** |
| **gemini-3.5-flash** | Not stated | ✅ valid | ✅ valid | 1,521 / 608 / 1,097 | 8.6–17.5 s | Good fallback |
| **gemini-3.1-flash-lite** | Yes | ✅ valid | ✅ valid | 1,521 / 589 / 0 | 3.4 s | Fastest and cheapest; check quality |
| gemini-4-hydrogen | Yes | ✅ valid | ❌ 503 | 2,177 / 584 / 1,512 | 11.1 s | Works when not overloaded |
| gemma-4-26b-a4b-it | Yes | ❌ 503 | ✅ valid | 997 / 370 / 0 | 10.2 s | Works when not overloaded; open model |
| gemini-3.8-flash | Yes | ❌ 503 | ❌ 503 | none | none | Overloaded on every attempt across both days |
| gemini-3.5-flash-lite | Yes | ❌ 503 | ❌ 503 | none | none | Answered the 11-22 probe; overloaded on 11-23 |
| gemma-4-31b-it | Yes | ❌ 503 | ❌ 503 | none | none | Overloaded on every attempt |
| gemini-3-flash-preview | Yes ("legacy") | ❌ 404 | ❌ 404 | none | none | Not on the v1 API our SDK uses |
| gemini-flash-latest, gemini-flash-lite-latest | — | ❌ 404 | ❌ 404 | none | none | Aliases exist only on v1beta |

"Free tier" is from Google's pricing page (https://aistudio.google.com/docs/pricing). Gemini 3.5 Flash's free status wasn't stated in what we could read, but it worked on the free key.

**Not usable:**
- `gemini-2.5-flash`, `-flash-lite` and `-pro` return 404: "no longer available to new users". The pricing page says that from 2026-12-01, API calls to 2.5 Flash route to 3.5 Flash-Lite and calls to 2.5 Pro route to 3.8 Flash.
- `gemini-4-neon`, `-argon` and `-carbon`, and `gemini-3.1-pro-preview`, are paid only. On the free key they return 429 with a free-tier quota of zero.

### Problems found and fixed

- **The schema was rejected.** Gemini returned 400 "The specified schema produces a constraint that has too many states for serving" when the outer `questions` array had both `minItems: 5` and `maxItems: 30`. With `minItems: 5` alone it's accepted. The upper bound (20) now lives in the prompt and in the domain validation. A unit test fails if `maxItems` comes back.
- **The default model was retired:** `gemini-2.5-flash` is closed to new users, so the default is now `gemini-3.8-flash`. Given the 503s above, consider switching the default to `gemini-3.6-flash`.

### Prompt behaviour (one sample)

We used a 3-page sample PDF mixing course admin (assignment deadline, late penalty, grade weighting, office hours, TA email, tutorial task) with hash-table lecture content, on `gemini-3.6-flash`:
- All 5 questions were about the lecture content and none touched the admin.
- One was a "why" question (tombstones in open addressing); a couple still leaned on recall.

One small sample proves little; judge quality on real lecture PDFs.

### Rate limits

- **Google doesn't publish the free-tier numbers per model.** They're only on the signed-in AI Studio page: https://aistudio.google.com/rate-limit.
- **What the docs do say** (https://aistudio.google.com/docs/rate-limits):
  - Limits are measured as requests per minute (RPM), input tokens per minute (TPM) and requests per day (RPD).
  - Limits apply "per project, not per API key".
  - The daily limit "reset[s] at midnight Pacific time".
- **Quota errors** name the free-tier quota types (requests per minute and per day, input tokens per minute and per day, per model) but don't include the values.
- **The free tier may use your data:** "Content used to improve our products." Uploaded lecture PDFs would be covered. The paid tier doesn't do this.

## Other providers' free tiers

Researched 2026-11-23 from official docs only; none were tested, since that needs keys. "Not stated" means the page we read didn't say. Our adapter needs:
1. **A recurring free allowance.**
2. **JSON Schema output**, ideally strict (enforced while the model writes).
3. **Text input** (`text-extract`), unless the provider takes PDFs. Only Gemini takes the PDF natively.

| Provider | Free access | Best free models | Free limits | JSON schema | PDF input | Data use | Fit |
|---|---|---|---|---|---|---|---|
| **Groq** | Free plan exists; its numbers weren't readable | `openai/gpt-oss-120b`, `gpt-oss-20b`, `qwen/qwen3.8-27b` (131k context) | The page's table is labelled "Developer plan": 30 RPM, 1K RPD, 8K TPM, 200K TPD for gpt-oss-120b. The models page contradicts it (250K TPM / 1K RPM). | Yes, strict (enforced) on gpt-oss and qwen | Not stated, so text only | Inputs not kept by default; logs kept up to 30 days; training not stated | Promising, but **8K TPM is too small** for one ~30-page lecture as text |
| **OpenRouter** | `:free` model variants | Couldn't list them; the free filter didn't render | 20 RPM; **50 RPD** (1,000 RPD after buying ≥10 credits) | Yes; strict where the underlying provider supports it | Yes, any model, via a PDF parser (`cloudflare-ai` engine is free) | Set per provider; free-model training is an account setting | Most flexible; 50 requests a day is fine for a personal app |
| **Cerebras** | **No permanent free tier.** A one-time $5 credit, 30 days, payment method required | `qwen-3.8-27b` (300 RPM, 150K TPM, no daily cap), `gpt-oss-120b` (5 RPM, 1M TPD) | See left | Yes, strict, but strict mode **doesn't support `minItems`/`maxItems`** | Not stated, so text only | Not stated | Trial only |
| **Cloudflare Workers AI** | 10,000 "neurons" per day, resets 00:00 UTC | `gpt-oss-120b`, `qwen3.8-27b`, `llama-3.3-70b-instruct-fp8-fast`, `nemotron-3-120b-a12b`, `gemma-4-26b-a4b-it` | 300 RPM. A 50k-token prompt costs ~1,800 neurons on the 70–120B models, about **5 calls a day** | `json_schema`, **not enforced** ("can't guarantee"); only listed models | Not stated, so text only | Not stated | Too small a daily budget for big models; same platform as our Worker |
| **Mistral** | Unverified: the free-tier page returns 404 | `mistral-small-2603` (256k), `ministral-8b-2512` (256k); free status not stated | Not stated | Custom structured output exists; whether it's enforced is unverified | Separate OCR endpoint (`mistral-ocr-4-1`) | Not stated | Needs a manual check |
| **GitHub Models** | **Retired 2026-07-30** | none | none | none | none | none | Ruled out |

Sources:
- Groq: https://console.groq.com/docs/rate-limits, https://console.groq.com/docs/structured-outputs, https://console.groq.com/docs/models, https://console.groq.com/docs/your-data
- OpenRouter: https://openrouter.ai/docs/api-reference/limits, https://openrouter.ai/docs/guides/features/structured-outputs, https://openrouter.ai/docs/guides/overview/multimodal/pdfs.md, https://openrouter.ai/docs/guides/privacy/provider-logging.md
- Cerebras: https://inference-docs.cerebras.ai/support/rate-limits, https://inference-docs.cerebras.ai/capabilities/structured-outputs
- Cloudflare: https://developers.cloudflare.com/workers-ai/platform/pricing/, https://developers.cloudflare.com/workers-ai/platform/limits/, https://developers.cloudflare.com/workers-ai/features/json-mode/
- Mistral: https://docs.mistral.ai/models.md (the free-tier and structured-output pages were 404)
- GitHub Models: https://docs.github.com/en/github-models/use-github-models/prototyping-with-ai-models

### What adding one would take

- **A new adapter** under `backend/internal/adapters/llm/<provider>/` plus an `LLM_PROVIDER` value. The port, domain and prompt stay as they are.
- **Text input:** use the existing `ledongthuc` text extractor (`text-extract`). Scanned PDFs won't work there, because they have no text layer.
- **A schema per provider:** strict modes differ.
  - Groq strict needs `additionalProperties: false` and every field in `required`. Our optional `sourceHint` would have to become required or nullable.
  - Cerebras strict rejects `minItems`/`maxItems`.
  - Since the domain already validates everything, an adapter can drop unsupported keywords safely.
- **Quota:** a 30-page lecture is roughly 10–50k tokens of text. Check it against each provider's tokens-per-minute limit, which is where Groq's 8K falls short.

## Open questions

- The per-model free-tier numbers for Gemini (AI Studio, signed in).
- Groq's free-plan tab, and whether it needs a card.
- OpenRouter's current `:free` models that support structured outputs (`supported_parameters=structured_outputs` on the models API).
- Question quality: compare `gemini-3.6-flash`, `3.5-flash` and `3.1-flash-lite` on the same real lecture PDFs, and run `make bench` for `pdf-native` against `text-extract`.
- **Resilience:** add retry with backoff on 503/429, and a fallback model, before relying on any free model in production.
