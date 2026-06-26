-- 013_add_webhook_id_default.up.sql
-- Sets a default value of 0 for webhook_id so new projects don't get NULL.
-- Also backfills existing projects with NULL webhook_id to 0.

UPDATE projects SET webhook_id = 0 WHERE webhook_id IS NULL;

ALTER TABLE projects ADD COLUMN webhook_id_new INTEGER NOT NULL DEFAULT 0;

UPDATE projects SET webhook_id_new = webhook_id;

ALTER TABLE projects DROP COLUMN webhook_id;

ALTER TABLE projects RENAME COLUMN webhook_id_new TO webhook_id;
