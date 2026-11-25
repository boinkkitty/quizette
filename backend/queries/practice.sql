-- Attempts (quizzes) and their graded answers. An attempt's scope is a topic or a
-- chapter; exactly one of topic_id and chapter_id is set (ADR 0010).

-- name: CreateAttempt :exec
INSERT INTO attempts (id, owner_id, scope, topic_id, chapter_id, requested_count, question_ids, total_score, max_score, started_at, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetAttempt :one
SELECT * FROM attempts WHERE owner_id = $1 AND id = $2;

-- name: InsertAnswer :exec
INSERT INTO attempt_answers (attempt_id, question_id, position, selected, points, correct)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAnswers :many
SELECT * FROM attempt_answers WHERE attempt_id = $1 ORDER BY position;

-- Chapter-scoped attempts only: a topic attempt has no chapter_id.
-- name: ListAttemptsByChapter :many
SELECT * FROM attempts WHERE owner_id = $1 AND chapter_id = $2 ORDER BY started_at DESC;

-- The History page: newest attempts across all scopes, with names for display. A
-- topic attempt has no chapter, so chapter_title is empty and the topic is its own.
-- name: ListAttemptsByOwner :many
SELECT a.*, coalesce(c.title, '')::text AS chapter_title, t.id AS scope_topic_id, t.name AS topic_name
FROM attempts a
LEFT JOIN chapters c ON c.id = a.chapter_id
JOIN topics t ON t.id = coalesce(a.topic_id, c.topic_id)
WHERE a.owner_id = $1
ORDER BY a.started_at DESC
LIMIT $2;
