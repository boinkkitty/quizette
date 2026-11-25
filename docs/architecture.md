# Quizette architecture

## 1. System overview
```
 Browser ──► Render: Go binary ──┬─► embedded static assets (React SPA)
                                 └─► /api/*  ──► Neon Postgres
                                        │
                                        └──► Gemini (QuestionGenerator port)
```
One origin for the browser (the Go binary serves both), so the HttpOnly session cookie is first-party and no CORS is needed (ADR 0004, ADR 0007, ADR 0009). The Go API is a single deployable (modular monolith) structured as ports & adapters.

## 2. Hexagonal layers
```
          driving adapters                       driven adapters
       ┌──────────────────┐                ┌──────────────────────────┐
HTTP ─►│ adapters/http    │                │ adapters/postgres (sqlc) │
       └────────┬─────────┘                │ adapters/llm/{gemini,stub}│
                ▼                          │ adapters/pdf/ledongthuc  │
       ┌──────────────────┐   ports        │ adapters/queue/inprocess │
       │ app (use cases)  │──────────────► │ adapters/oauth           │
       └────────┬─────────┘ (interfaces    └──────────────────────────┘
                ▼            owned by app)
       ┌──────────────────┐
       │ domain           │  stdlib only
       └──────────────────┘
```
- **Dependency rule:** `domain ← app ← adapters ← cmd`. Enforced by `depguard` + an architecture test.
- **Why hexagonal here:** the volatile parts are exactly the edges — LLM provider (Gemini today, maybe Claude/OpenAI/Ollama), PDF input strategy (native vs extracted text, under active experiment), job execution (in-process now, durable queue later). Ports make each swappable without touching rules about questions, grading, or attempts. The core rules (question validity, MRQ grading, selection) are pure and tested without I/O.
- **Why not hexagonal everywhere:** Identity (users, sessions, OAuth) is a generic subdomain — same ports/adapters layering, but no rich aggregates; it's essentially CRUD + crypto.

## 3. Aggregate design principles used
1. An aggregate is a **consistency boundary**: everything that must change atomically to keep an invariant true lives inside it; nothing else does.
2. **Small aggregates**, **reference others by ID** — avoids loading big graphs and lock contention.
3. Cross-aggregate changes that must be atomic (e.g. job succeeds + question set saved + chapter ready) go through one `UnitOfWork` transaction in the application layer — pragmatic for a single-DB monolith instead of eventual consistency.
4. Set-wide uniqueness (e.g. topic name per user) is enforced by DB constraints, surfaced as domain errors.

## 4. Aggregate map
```
Topic 1 ──< Chapter 1 ── 1 QuestionSet 1 ──< Question
  │            │  1                               ▲ (by ID)
  │            ├──< ProcessingJob ··· Document     │
  │            │      (metadata only) (transient)  │
  │            └──< Attempt ──< Answer ────────────┘
  └──────────────< Attempt   (scope: chapter or topic)
```
`──` ownership/ID reference · `···` used in-memory only, never persisted.

## 5. Aggregates & key types — justification

### Topic (aggregate root)
- **What:** user-named grouping of chapters. `ID, OwnerID, Name`.
- **Why its own aggregate:** chapters are created, renamed, failed and deleted independently; putting Chapters *inside* Topic would force loading every chapter to add one and serialise concurrent uploads in the same topic. The only Topic invariant (non-empty, unique name per owner) doesn't involve chapters.
- **Rejected:** Topic as a mere string tag on Chapter — loses rename-once semantics and an empty topic can't exist.

### Chapter (aggregate root)
- **What:** the user-visible unit created by one upload. `ID, TopicID, OwnerID, Title, Status(Pending|Ready|Failed)`.
- **Why separate from QuestionSet:** a Chapter exists *before* any questions (Pending while generating, Failed if generation fails) and has mutable metadata (title, topic move). QuestionSet is immutable content. Splitting keeps "organising" and "content" from blocking each other and leaves room for regenerating a set later without changing the chapter's identity/history.
- **Why separate from ProcessingJob:** Chapter answers "what does the user have?"; the job answers "what happened during processing?" (attempts, errors, tokens, timings). Keeping job noise out of Chapter keeps it a stable read model for the UI.
- **Trade-off acknowledged:** Chapter+QuestionSet could be merged (1:1 today). Kept apart because their lifecycles and mutability differ; cost is one extra table/join.

### Document (value object — never persisted)
- **What:** the uploaded PDF in memory. `Filename, MediaType, Bytes, SizeBytes, SHA256`.
- **Why value object:** no identity or lifecycle we track; two identical uploads are interchangeable (equality by content hash). `NewDocument` validates (PDF magic bytes, ≤15 MB) so an invalid Document can't exist.
- **Why first-class at all:** it's the input to the `QuestionGenerator`/`TextExtractor` ports and the carrier of the **"PDFs are never stored"** policy — making it an explicit type means the rule is visible in code: there is no repository for Document, only metadata copied onto the job.

### ProcessingJob (aggregate root)
- **What:** one attempt to turn a Document into a QuestionSet. `ID, ChapterID, OwnerID, Spec, DocumentMeta, Status, Attempts, Error, Metrics, timestamps`.
- **Why an aggregate:** it has a real **state machine** with invariants (`Queued → Running → Succeeded|Failed`, no transitions out of terminal states, can't succeed without a set). Encoding that as methods (`Start/Succeed/Fail`) prevents the "status column anyone can write" bug class.
- **Why separate:** it's written by the background worker while the user polls/reads; isolating it avoids contention with Chapter/QuestionSet. Also the home for **generation metrics** (strategy, model, tokens, timings) — required for the PDF-vs-text experiment and ongoing cost tracking.
- **Why `Chapter 1──< Job` (not 1:1):** allows retries/regeneration later without schema change.

### QuestionSpec (value object)
- **What:** `Count (5..20, multiple of 5), TypeMix`.
- **Why:** the "multiple of 5 between 5 and 20" rule is needed at upload, in the job, in the LLM prompt and when validating output — one type means one place for the rule.

### QuestionSet (aggregate root) + Question (entity inside)
- **What:** the chapter's bank. `ID, ChapterID, Questions[]`.
- **Why QuestionSet is the root:** invariants span the **collection** — exact count equals the spec, unique positions, every question valid. Questions are created together from one LLM response and never edited individually (v1), so they share one consistency boundary.
- **Why the factory is the anti-corruption layer:** `NewQuestionSet(chapterID, spec, drafts)` is the only way LLM output becomes domain data. The JSON schema guarantees *shape*; the factory guarantees *meaning* (MCQ/TF one answer, MRQ 3–5 options with at least 1 correct, indices in range, no duplicate options, TF options exactly True/False). Swapping LLMs can't weaken these rules.
- **Why immutable:** attempts reference question IDs; if questions could change, past scores would silently become wrong.
- **Why Question is an entity (has ID) and not a value object:** attempts refer to specific questions over time, which requires stable identity.
- **Why `Grade()` lives on Question:** grading depends only on the question's type and correct answer — behaviour next to data, no type-switching scattered across services.

### Score (value object) & MRQ grading
- `Points ∈ [0,1], Correct bool`. MCQ/TF: 1 or 0.
- **MRQ:** `max(0, (correctSelected − wrongSelected) / totalCorrect)`.

| Alternative | Problem |
|---|---|
| All-or-nothing | Too harsh; no signal of partial understanding |
| `correctSelected / totalCorrect` | Select-all scores 100% |
| Per-option right/wrong over all options | Selecting nothing scores well when few options are correct |
| **Chosen** | Rewards each correct pick, cancels guesses, floor at 0 so one question can't drag total negative |

Example: correct = {A,C}. Pick A,C → 1. Pick A → 0.5. Pick A,B → 0. Pick all 4 → 0.

### Attempt (aggregate root)
- **What:** one finished practice run over an attempt scope: one chapter, or a whole topic (its ready chapters) (ADR 0010). `ID, OwnerID, Scope{Type, ID}, RequestedCount, QuestionIDs (ordered), Answers[], TotalScore, MaxScore, StartedAt, SubmittedAt`. Created graded when the quiz is submitted; an unfinished quiz lives only in its browser tab (ADR 0011).
- **Why its own aggregate:** many attempts per set, written by the user while the set is read-only. Putting attempts inside QuestionSet would make the immutable bank a write hotspot and grow unboundedly.
- **Invariants:** count is a multiple of 5, at least 5 and ≤ min(pool size, 50), where the pool (`domain.Pool`) is the question sets of the scope's ready chapters; drawn questions are distinct members of the pool; answers only for drawn questions; score derived, never client-supplied.
- **Snapshot of `QuestionIDs`:** history shows exactly what was asked, in what order, even though selection is randomised, and chapters added to a topic later don't change its past attempts. An answer's chapter is derived through question → question set → chapter.
- **Stats:** a chapter's attempt count and best score count chapter-scoped attempts only; a topic's count, last and best score count topic-scoped attempts only.
- **Name:** "Attempt" with a scope rather than a ChapterAttempt per chapter (ADR 0010); "Quiz" is the UI word only.

### DrawQuestions, SplitAcrossChapters (domain services)
- `SplitAcrossChapters` splits N across a topic's ready chapters in proportion to their question counts, with largest-remainder rounding; remainder ties go to the earlier-created chapter. The frontend's split preview must match it.
- `DrawQuestions` checks the count rule against the pool, draws each chapter's share at random, then shuffles the whole draw so chapters are mixed. A chapter scope is a pool of one set. Stateless: the draw is returned to the client and not stored until the quiz is submitted (ADR 0011).

### User / Session (Identity — not rich aggregates)
- Plain entities behind repositories; rules are mostly crypto/protocol (bcrypt, OAuth state/PKCE), not domain behaviour.
- **Session cookie** (ADR 0007): an opaque 32-byte random token in `qz_session` (`HttpOnly; Secure; SameSite=Lax; Path=/`, 30 days). Only its SHA-256 is stored, as a `Session` row.
- **Request context:** the HTTP middleware calls `IdentityService.Authenticate` (session row + user) and stores the `domain.User` in `context.Context`. Handlers read it with `mustUser(r)`. Every use case takes the owner ID explicitly, so the app layer never reads the context itself.
- **CSRF:** every mutating `/api/*` request needs `X-Requested-With`; browsers can't add it cross-site without CORS, which is never enabled.

## 6. Persistence schema — justification
| Table | Key decisions |
|---|---|
| `users` | `email citext unique` → case-insensitive login/OAuth email linking; `password_hash` nullable for OAuth-only users. |
| `oauth_accounts` | `unique(provider, provider_user_id)`; one user can link Google + GitHub. |
| `sessions` | One row per live login; PK is the **SHA-256** of the session token, not the token — a DB leak doesn't yield live sessions. Deleted on logout; expired rows purged hourly. |
| `topics` | `unique(owner_id, name)` enforces the Topic invariant cheaply. |
| `chapters` | `owner_id` denormalised (also derivable via topic) so every query filters by owner without joins → authorization at query level. |
| `processing_jobs` | Only `doc_filename, doc_size, doc_sha256` — **no bytes column**, the no-storage policy is visible in the schema. sha256 enables future dedupe/caching. Metric columns (`strategy, model, *_tokens, extract_ms, llm_ms, total_ms, extracted_chars, validation_retries`) for the experiment and cost tracking. |
| `question_sets` | `chapter_id UNIQUE` enforces 1:1 today; dropping the constraint later enables versioned regeneration. |
| `questions` | `options jsonb` (ordered list, always read/written with the question, never queried alone — a child table adds joins for nothing); `correct int[]` = indices into options; `position` for stable order. |
| `attempts` | `scope` with exactly one of `topic_id` / `chapter_id` set (CHECK constraint); `question_ids uuid[]` snapshot of draw order; indexes `(owner_id, chapter_id, started_at desc)` and `(owner_id, topic_id, started_at desc)` for history. |
| `attempt_answers` | Real table (not jsonb), indexed `(question_id)`. `points numeric(5,4)` for MRQ fractions. |

Cross-cutting:
- **UUIDv7** IDs generated in the app (`IDGenerator` port) → domain objects have identity before persistence; time-ordered for index locality.
- **Status as `text` + `CHECK`** rather than Postgres `ENUM` — enums are awkward to alter in migrations.
- **`ON DELETE CASCADE`** topic → chapters → (jobs, question_sets → questions, attempts → answers): deleting a chapter cleanly removes its content and history.
- **sqlc** keeps SQL explicit and typed; mappers in the Postgres adapter translate rows ↔ domain so the domain never sees DB types.

## 7. Key flow — upload to question set
```
HTTP POST /api/documents
  → app.UploadDocument: NewDocument, QuestionSpec; UoW{ Topic?, Chapter(Pending), Job(Queued) }; JobQueue.Enqueue(job, doc)
  ← 202 {jobId, chapterId}
worker → app.ProcessJob: job.Start()
  → QuestionGenerator.Generate(doc, spec)  [pdf-native | text-extract via TextExtractor]  (schema-constrained JSON)
  → NewQuestionSet(...)  (retry once on invariant violation)
  → UoW{ save QuestionSet, job.Succeed(metrics), chapter.MarkReady() }   | on error: job.Fail, chapter.MarkFailed
  Document bytes dropped (GC) — never written anywhere.
client polls GET /api/jobs/:id
```

## 8. Implementation notes

- **Exported fields, guarded transitions.** Aggregates use exported fields (idiomatic Go, simple mapping in the Postgres adapter), but state changes go through methods (`ProcessingJob.Start/Succeed/Fail`, `Chapter.MarkReady/MarkFailed`) that enforce the state machines, and `NewAttempt` only builds an attempt that is already graded.
- **Lenient anti-corruption.** `NewQuestionSet` discards invalid LLM drafts and succeeds if at least `spec.Count` valid ones remain; only if too few are valid does the job retry once with the violations fed back into the prompt.
- **Uploads never touch disk.** The HTTP adapter streams multipart parts into memory instead of using `ParseMultipartForm` (which spills large files to temp files). A schema test asserts there is no `bytea` column anywhere.
- **Neon / PgBouncer.** The pool uses `QueryExecModeExec` (no server-side prepared statements). In this mode pgx cannot infer parameter types, so `jsonb` is mapped to Go `string` (a `[]byte` would be sent as `bytea`).
- **Request-scoped logging survives into jobs.** The in-process queue runs jobs with `context.WithoutCancel(requestCtx)`, keeping the request's logger (`request_id`, `user_id`) while detaching from its cancellation; shutdown cancels via `context.AfterFunc`.

## 9. Where things live

| Concern | Path |
|---|---|
| Domain model | `backend/internal/domain` |
| Use cases + ports | `backend/internal/app` (`ports.go`, `question_set.schema.json`, `prompt.go`) |
| In-memory port fakes | `backend/internal/app/apptest` |
| Adapters | `backend/internal/adapters/{http,postgres,llm/gemini,llm/stub,pdf/ledongthuc,queue/inprocess,oauth,system}` |
| Composition root | `backend/cmd/api/main.go` |
| Strategy benchmark | `backend/cmd/genbench` |
| SPA | `frontend/src` (routes, `api/schemas.ts` zod contract) |
| Embedded SPA | `backend/web` (filled by the root `Dockerfile`), served by `backend/internal/adapters/http/spa.go` |
| E2E | `e2e/tests` |
