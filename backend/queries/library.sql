-- Topics and chapters. Every query filters by owner_id, which is how ownership is enforced.
-- Deleted topics and chapters (deleted_at set, ADR 0010) are hidden: every query skips
-- them unless its comment says otherwise.

-- name: CreateTopic :exec
INSERT INTO topics (id, owner_id, name, created_at) VALUES ($1, $2, $3, $4);

-- name: GetTopic :one
SELECT * FROM topics WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- Includes deleted topics: a past attempt shows its topic's name.
-- name: GetTopicIncludingDeleted :one
SELECT * FROM topics WHERE owner_id = $1 AND id = $2;

-- name: GetTopicByName :one
SELECT * FROM topics WHERE owner_id = $1 AND name = $2 AND deleted_at IS NULL;

-- Performance counts topic-scoped attempts only. The score columns are -1 when there
-- are none (mapped to nil).
-- name: ListTopics :many
SELECT t.*, (SELECT count(*) FROM chapters c WHERE c.topic_id = t.id AND c.deleted_at IS NULL)::int AS chapter_count,
    coalesce((SELECT count(*) FROM attempts a WHERE a.topic_id = t.id), 0)::int AS attempt_count,
    coalesce((SELECT a.total_score / a.max_score * 100 FROM attempts a WHERE a.topic_id = t.id ORDER BY a.submitted_at DESC, a.id DESC LIMIT 1), -1)::float8 AS last_score_pct,
    coalesce((SELECT max(a.total_score / a.max_score * 100) FROM attempts a WHERE a.topic_id = t.id), -1)::float8 AS best_score_pct
FROM topics t
WHERE t.owner_id = $1 AND t.deleted_at IS NULL
ORDER BY t.name;

-- name: UpdateTopic :execrows
UPDATE topics SET name = $3 WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: DeleteTopic :execrows
DELETE FROM topics WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: MarkTopicDeleted :execrows
UPDATE topics SET deleted_at = $3 WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: MarkTopicChaptersDeleted :exec
UPDATE chapters SET deleted_at = $3 WHERE owner_id = $1 AND topic_id = $2 AND deleted_at IS NULL;

-- name: CreateChapter :exec
INSERT INTO chapters (id, topic_id, owner_id, title, status, created_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetChapter :one
SELECT * FROM chapters WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- Includes deleted chapters: generation jobs look their chapter up by ID alone.
-- name: GetChapterByID :one
SELECT * FROM chapters WHERE id = $1;

-- Chapters of one topic with list-view aggregates. The aggregates are correlated
-- subqueries rather than joins so one chapter row can't be multiplied by its questions
-- and attempts. Attempt figures count chapter-scoped attempts only (a topic attempt has
-- no chapter_id). best_score_pct is -1 when there are none (mapped to nil).
-- name: ListChaptersByTopic :many
SELECT c.*, t.name AS topic_name,
    coalesce((SELECT count(*) FROM questions q JOIN question_sets s ON s.id = q.question_set_id WHERE s.chapter_id = c.id), 0)::int AS question_count,
    coalesce((SELECT count(*) FROM attempts a WHERE a.chapter_id = c.id), 0)::int AS attempt_count,
    coalesce((SELECT max(a.total_score / a.max_score * 100) FROM attempts a WHERE a.chapter_id = c.id), -1)::float8 AS best_score_pct
FROM chapters c
JOIN topics t ON t.id = c.topic_id
WHERE c.owner_id = $1 AND c.topic_id = $2 AND c.deleted_at IS NULL
ORDER BY c.created_at, c.id;

-- Same columns as ListChaptersByTopic (the Go adapter converts between the two row types),
-- across all of a user's topics. The optional filters are skipped when NULL; the explicit
-- casts are needed because the Neon pooler runs pgx in exec mode, which can't infer types.
-- name: ListChaptersByOwner :many
SELECT c.*, t.name AS topic_name,
    coalesce((SELECT count(*) FROM questions q JOIN question_sets s ON s.id = q.question_set_id WHERE s.chapter_id = c.id), 0)::int AS question_count,
    coalesce((SELECT count(*) FROM attempts a WHERE a.chapter_id = c.id), 0)::int AS attempt_count,
    coalesce((SELECT max(a.total_score / a.max_score * 100) FROM attempts a WHERE a.chapter_id = c.id), -1)::float8 AS best_score_pct
FROM chapters c
JOIN topics t ON t.id = c.topic_id
WHERE c.owner_id = sqlc.arg('owner_id') AND c.deleted_at IS NULL
    AND (sqlc.narg('topic_id')::uuid IS NULL OR c.topic_id = sqlc.narg('topic_id')::uuid)
    AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text)
ORDER BY t.name, c.created_at;

-- Title and status are saved separately, each touching only its own column, so a rename
-- during generation can't overwrite the job's status change (or the other way round).
-- name: UpdateChapterTitle :execrows
UPDATE chapters SET title = $3 WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: UpdateChapterStatus :execrows
UPDATE chapters SET status = $2 WHERE id = $1;

-- name: DeleteChapter :execrows
DELETE FROM chapters WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: MarkChapterDeleted :execrows
UPDATE chapters SET deleted_at = $3 WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL;

-- Includes deleted chapters: a past attempt shows questions from them.
-- name: ListChaptersByQuestions :many
SELECT c.* FROM chapters c
WHERE c.owner_id = $1 AND EXISTS (
    SELECT 1 FROM questions q JOIN question_sets s ON s.id = q.question_set_id
    WHERE s.chapter_id = c.id AND q.id = ANY(sqlc.arg('question_ids')::uuid[])
);
