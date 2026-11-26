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
-- Deleted chapters and topics are included and flagged by scope_deleted.
-- chapter_count is the number of distinct chapters the attempt drew questions from.
-- name: ListAttemptsByOwner :many
SELECT a.*, coalesce(c.title, '')::text AS chapter_title, t.id AS scope_topic_id, t.name AS topic_name,
    (CASE WHEN a.scope = 'chapter' THEN c.deleted_at IS NOT NULL ELSE t.deleted_at IS NOT NULL END)::bool AS scope_deleted,
    (SELECT count(DISTINCT s.chapter_id) FROM questions q
        JOIN question_sets s ON s.id = q.question_set_id
        WHERE q.id = ANY(a.question_ids))::int AS chapter_count
FROM attempts a
LEFT JOIN chapters c ON c.id = a.chapter_id
JOIN topics t ON t.id = coalesce(a.topic_id, c.topic_id)
WHERE a.owner_id = $1
ORDER BY a.started_at DESC
LIMIT $2;

-- Whether any of the user's attempts drew a question from the chapter. Every drawn
-- question has an answer row, answered or not, so the answers index finds it.
-- name: ChapterReferenced :one
SELECT EXISTS (
    SELECT 1 FROM attempt_answers aa
    JOIN attempts a ON a.id = aa.attempt_id
    JOIN questions q ON q.id = aa.question_id
    JOIN question_sets s ON s.id = q.question_set_id
    WHERE a.owner_id = $1 AND s.chapter_id = $2
);

-- Whether any of the user's attempts is scoped to the topic or drew a question from
-- one of its chapters, deleted or not.
-- name: TopicReferenced :one
SELECT (
    EXISTS (SELECT 1 FROM attempts a WHERE a.owner_id = sqlc.arg('owner_id') AND a.topic_id = sqlc.arg('topic_id')::uuid)
    OR EXISTS (
        SELECT 1 FROM attempt_answers aa
        JOIN attempts a ON a.id = aa.attempt_id
        JOIN questions q ON q.id = aa.question_id
        JOIN question_sets s ON s.id = q.question_set_id
        JOIN chapters c ON c.id = s.chapter_id
        WHERE a.owner_id = sqlc.arg('owner_id') AND c.topic_id = sqlc.arg('topic_id')::uuid
    )
)::bool AS referenced;
