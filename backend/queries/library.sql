-- Topics and chapters. Every query filters by owner_id, which is how ownership is enforced.

-- name: CreateTopic :exec
INSERT INTO topics (id, owner_id, name, created_at) VALUES ($1, $2, $3, $4);

-- name: GetTopic :one
SELECT * FROM topics WHERE owner_id = $1 AND id = $2;

-- name: GetTopicByName :one
SELECT * FROM topics WHERE owner_id = $1 AND name = $2;

-- name: ListTopics :many
SELECT t.*, (SELECT count(*) FROM chapters c WHERE c.topic_id = t.id)::int AS chapter_count
FROM topics t
WHERE t.owner_id = $1
ORDER BY t.name;

-- name: UpdateTopic :execrows
UPDATE topics SET name = $3 WHERE owner_id = $1 AND id = $2;

-- name: DeleteTopic :execrows
DELETE FROM topics WHERE owner_id = $1 AND id = $2;

-- name: CreateChapter :exec
INSERT INTO chapters (id, topic_id, owner_id, title, status, created_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetChapter :one
SELECT * FROM chapters WHERE owner_id = $1 AND id = $2;

-- name: GetChapterByID :one
SELECT * FROM chapters WHERE id = $1;

-- Chapters of one topic with list-view aggregates. The aggregates are correlated
-- subqueries rather than joins so one chapter row can't be multiplied by its questions
-- and attempts. best_score_pct is -1 when there are no submitted attempts (mapped to nil).
-- name: ListChaptersByTopic :many
SELECT c.*, t.name AS topic_name,
    coalesce((SELECT count(*) FROM questions q JOIN question_sets s ON s.id = q.question_set_id WHERE s.chapter_id = c.id), 0)::int AS question_count,
    coalesce((SELECT count(*) FROM chapter_attempts a WHERE a.chapter_id = c.id AND a.status = 'submitted'), 0)::int AS attempt_count,
    coalesce((SELECT max(a.total_score / a.max_score * 100) FROM chapter_attempts a WHERE a.chapter_id = c.id AND a.status = 'submitted'), -1)::float8 AS best_score_pct
FROM chapters c
JOIN topics t ON t.id = c.topic_id
WHERE c.owner_id = $1 AND c.topic_id = $2
ORDER BY c.created_at;

-- Same columns as ListChaptersByTopic (the Go adapter converts between the two row types),
-- across all of a user's topics. The optional filters are skipped when NULL; the explicit
-- casts are needed because the Neon pooler runs pgx in exec mode, which can't infer types.
-- name: ListChaptersByOwner :many
SELECT c.*, t.name AS topic_name,
    coalesce((SELECT count(*) FROM questions q JOIN question_sets s ON s.id = q.question_set_id WHERE s.chapter_id = c.id), 0)::int AS question_count,
    coalesce((SELECT count(*) FROM chapter_attempts a WHERE a.chapter_id = c.id AND a.status = 'submitted'), 0)::int AS attempt_count,
    coalesce((SELECT max(a.total_score / a.max_score * 100) FROM chapter_attempts a WHERE a.chapter_id = c.id AND a.status = 'submitted'), -1)::float8 AS best_score_pct
FROM chapters c
JOIN topics t ON t.id = c.topic_id
WHERE c.owner_id = sqlc.arg('owner_id')
    AND (sqlc.narg('topic_id')::uuid IS NULL OR c.topic_id = sqlc.narg('topic_id')::uuid)
    AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text)
ORDER BY t.name, c.created_at;

-- Title and status are saved separately, each touching only its own column, so a rename
-- during generation can't overwrite the job's status change (or the other way round).
-- name: UpdateChapterTitle :execrows
UPDATE chapters SET title = $3 WHERE owner_id = $1 AND id = $2;

-- name: UpdateChapterStatus :execrows
UPDATE chapters SET status = $2 WHERE id = $1;

-- name: DeleteChapter :execrows
DELETE FROM chapters WHERE owner_id = $1 AND id = $2;
