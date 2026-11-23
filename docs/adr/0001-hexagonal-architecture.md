# 1. Hexagonal architecture with a DDD domain core

Status: accepted (2026-11-20)

## Context
The volatile parts of Quizette are at its edges: the LLM provider (Gemini free tier today), how a PDF is fed to it (under experiment), and how jobs run (in-process now, maybe a durable queue later). The core rules — question validity, MRQ grading, question selection, job state — are stable and must not depend on any of those.

## Decision
Ports & adapters. `internal/domain` (stdlib only) ← `internal/app` (use cases + port interfaces) ← `internal/adapters/*` ← `cmd/*` (composition root). Aggregates: Topic, Chapter, ProcessingJob, QuestionSet (with Question entities), ChapterAttempt; Document and QuestionSpec are value objects. Identity (users, sessions, OAuth) uses the same layering without rich aggregates. The rule is enforced by `depguard` and `internal/architecture_test.go`.

## Consequences
Use cases are tested with in-memory fakes (`app/apptest`), adapters against real Postgres. More files and mapping code than a CRUD layout; accepted for swappable edges and pure, fast core tests.
