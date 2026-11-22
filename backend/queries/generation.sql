-- Processing jobs and question sets. Every user-facing query filters by owner_id;
-- the *ByID variants are for the background worker, which acts on behalf of the owner.

-- name: CreateJob :exec
INSERT INTO processing_jobs (
    id, chapter_id, owner_id, requested_count, type_mix, doc_filename, doc_size, doc_sha256,
    status, attempts, error, queued_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetJob :one
SELECT * FROM processing_jobs WHERE owner_id = $1 AND id = $2;

-- name: GetJobByID :one
SELECT * FROM processing_jobs WHERE id = $1;

-- The newest job explains a chapter's current status (e.g. why it failed).
-- name: LatestJobForChapter :one
SELECT * FROM processing_jobs WHERE owner_id = $1 AND chapter_id = $2 ORDER BY queued_at DESC LIMIT 1;

-- name: UpdateJob :execrows
UPDATE processing_jobs SET
    status = $2, attempts = $3, error = $4,
    strategy = $5, model = $6, input_tokens = $7, output_tokens = $8, thinking_tokens = $9, total_tokens = $10,
    extract_ms = $11, llm_ms = $12, total_ms = $13, extracted_chars = $14, validation_retries = $15,
    started_at = $16, finished_at = $17
WHERE id = $1;

-- Uploads since a time, for the per-user daily limit (uses processing_jobs_owner_queued_idx).
-- name: CountJobsSince :one
SELECT count(*)::int FROM processing_jobs WHERE owner_id = $1 AND queued_at >= $2;

-- Jobs a restart interrupted; the API marks them failed on boot (uses the partial index).
-- name: ListUnfinishedJobs :many
SELECT * FROM processing_jobs WHERE status IN ('queued', 'running') ORDER BY queued_at;

-- name: CreateQuestionSet :exec
INSERT INTO question_sets (id, chapter_id, created_at) VALUES ($1, $2, $3);

-- name: CreateQuestion :exec
INSERT INTO questions (id, question_set_id, position, type, prompt, options, correct, explanation, source_hint)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetQuestionSetByChapter :one
SELECT * FROM question_sets WHERE chapter_id = $1;

-- name: ListQuestionsBySet :many
SELECT * FROM questions WHERE question_set_id = $1 ORDER BY position;
