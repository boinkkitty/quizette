-- Attempts have an attempt scope (ADR 0010): a whole topic or one chapter, with
-- exactly one of topic_id and chapter_id set. Every existing attempt is chapter-scoped.

-- +goose Up
ALTER TABLE chapter_attempts RENAME TO attempts;
ALTER TABLE chapter_attempt_answers RENAME TO attempt_answers;
ALTER INDEX chapter_attempts_pkey RENAME TO attempts_pkey;
ALTER INDEX chapter_attempts_history_idx RENAME TO attempts_chapter_history_idx;
ALTER INDEX chapter_attempt_answers_pkey RENAME TO attempt_answers_pkey;
ALTER INDEX chapter_attempt_answers_question_idx RENAME TO attempt_answers_question_idx;

ALTER TABLE attempts ADD COLUMN scope text NOT NULL DEFAULT 'chapter' CHECK (scope IN ('topic', 'chapter'));
ALTER TABLE attempts ALTER COLUMN scope DROP DEFAULT;
ALTER TABLE attempts ADD COLUMN topic_id uuid REFERENCES topics (id) ON DELETE CASCADE;
ALTER TABLE attempts ALTER COLUMN chapter_id DROP NOT NULL;
ALTER TABLE attempts ADD CONSTRAINT attempts_scope_target_check CHECK (
    (scope = 'topic' AND topic_id IS NOT NULL AND chapter_id IS NULL) OR
    (scope = 'chapter' AND chapter_id IS NOT NULL AND topic_id IS NULL)
);
CREATE INDEX attempts_topic_history_idx ON attempts (owner_id, topic_id, started_at DESC) WHERE topic_id IS NOT NULL;

-- +goose Down
-- Topic-scoped attempts have no chapter to belong to, so they are deleted with their answers.
DELETE FROM attempts WHERE scope = 'topic';
DROP INDEX attempts_topic_history_idx;
ALTER TABLE attempts DROP CONSTRAINT attempts_scope_target_check;
ALTER TABLE attempts ALTER COLUMN chapter_id SET NOT NULL;
ALTER TABLE attempts DROP COLUMN topic_id;
ALTER TABLE attempts DROP COLUMN scope;

ALTER INDEX attempt_answers_question_idx RENAME TO chapter_attempt_answers_question_idx;
ALTER INDEX attempt_answers_pkey RENAME TO chapter_attempt_answers_pkey;
ALTER INDEX attempts_chapter_history_idx RENAME TO chapter_attempts_history_idx;
ALTER INDEX attempts_pkey RENAME TO chapter_attempts_pkey;
ALTER TABLE attempt_answers RENAME TO chapter_attempt_answers;
ALTER TABLE attempts RENAME TO chapter_attempts;
