-- Chapter attempts (quizzes) and their graded answers.

-- name: CreateAttempt :exec
INSERT INTO chapter_attempts (id, owner_id, chapter_id, requested_count, question_ids, status, total_score, max_score, started_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetAttempt :one
SELECT * FROM chapter_attempts WHERE owner_id = $1 AND id = $2;

-- name: UpdateAttempt :execrows
UPDATE chapter_attempts SET status = $2, total_score = $3, submitted_at = $4 WHERE id = $1;

-- name: InsertAnswer :exec
INSERT INTO chapter_attempt_answers (attempt_id, question_id, position, selected, points, correct)
VALUES ($1, $2, $3, $4, $5, $6);

-- Answers are replaced as a whole when an attempt is saved.
-- name: DeleteAnswers :exec
DELETE FROM chapter_attempt_answers WHERE attempt_id = $1;

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

-- Per-question history for SelectQuestions: how often each question was answered and the
-- points from the most recent submitted answer ("previously wrong" = last_points < 1).
-- name: QuestionStats :many
SELECT ans.question_id,
    count(*)::int AS attempts,
    ((array_agg(ans.points ORDER BY a.submitted_at DESC))[1])::float8 AS last_points
FROM chapter_attempt_answers ans
JOIN chapter_attempts a ON a.id = ans.attempt_id
WHERE a.owner_id = $1 AND a.chapter_id = $2 AND a.status = 'submitted'
GROUP BY ans.question_id;
