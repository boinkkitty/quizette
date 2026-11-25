-- Chapter attempts (quizzes) and their graded answers.

-- name: CreateAttempt :exec
INSERT INTO chapter_attempts (id, owner_id, chapter_id, requested_count, question_ids, total_score, max_score, started_at, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetAttempt :one
SELECT * FROM chapter_attempts WHERE owner_id = $1 AND id = $2;

-- name: InsertAnswer :exec
INSERT INTO chapter_attempt_answers (attempt_id, question_id, position, selected, points, correct)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAnswers :many
SELECT * FROM chapter_attempt_answers WHERE attempt_id = $1 ORDER BY position;

-- name: ListAttemptsByChapter :many
SELECT * FROM chapter_attempts WHERE owner_id = $1 AND chapter_id = $2 ORDER BY started_at DESC;

-- The History page: newest attempts across all chapters, with names for display.
-- name: ListAttemptsByOwner :many
SELECT a.*, c.title AS chapter_title, c.topic_id, t.name AS topic_name
FROM chapter_attempts a
JOIN chapters c ON c.id = a.chapter_id
JOIN topics t ON t.id = c.topic_id
WHERE a.owner_id = $1
ORDER BY a.started_at DESC
LIMIT $2;
