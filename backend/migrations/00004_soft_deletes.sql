-- Deleted chapters and topics (ADR 0010). Deleting a chapter or topic that an attempt
-- refers to sets deleted_at instead of removing the row, so the attempt keeps its
-- questions. Deleted rows are hidden everywhere and cannot be restored. A deleted
-- topic's name is free for a new topic, so uniqueness covers live topics only.

-- +goose Up
ALTER TABLE chapters ADD COLUMN deleted_at timestamptz;
ALTER TABLE topics ADD COLUMN deleted_at timestamptz;
ALTER TABLE topics DROP CONSTRAINT topics_owner_id_name_key;
CREATE UNIQUE INDEX topics_owner_name_live_idx ON topics (owner_id, name) WHERE deleted_at IS NULL;

-- +goose Down
-- Deleted content was only kept for its attempts, so it goes with them.
DELETE FROM topics WHERE deleted_at IS NOT NULL;
DELETE FROM chapters WHERE deleted_at IS NOT NULL;
DROP INDEX topics_owner_name_live_idx;
ALTER TABLE topics ADD CONSTRAINT topics_owner_id_name_key UNIQUE (owner_id, name);
ALTER TABLE topics DROP COLUMN deleted_at;
ALTER TABLE chapters DROP COLUMN deleted_at;
