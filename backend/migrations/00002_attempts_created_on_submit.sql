-- Attempts are created on submit (ADR 0011): an unfinished quiz lives only in its
-- browser tab, so the server has no in-progress attempts any more.

-- +goose Up
-- In-progress attempts can no longer be resumed or submitted. Their answers go with
-- them (ON DELETE CASCADE). Submitted attempts are kept as they are.
DELETE FROM chapter_attempts WHERE status = 'in_progress';
ALTER TABLE chapter_attempts DROP COLUMN status;
ALTER TABLE chapter_attempts ALTER COLUMN submitted_at SET NOT NULL;

-- +goose Down
-- Every remaining attempt was submitted. Deleted in-progress attempts are not restored.
ALTER TABLE chapter_attempts ALTER COLUMN submitted_at DROP NOT NULL;
ALTER TABLE chapter_attempts ADD COLUMN status text NOT NULL DEFAULT 'submitted' CHECK (status IN ('in_progress', 'submitted'));
ALTER TABLE chapter_attempts ALTER COLUMN status DROP DEFAULT;
