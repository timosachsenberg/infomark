BEGIN;

DROP VIEW IF EXISTS effective_submissions;
DROP VIEW IF EXISTS submission_participants;
DROP INDEX IF EXISTS submissions_user_id_idx;
DROP INDEX IF EXISTS submissions_team_id_idx;
DROP INDEX IF EXISTS submissions_task_id_team_id_key;
ALTER TABLE submissions DROP COLUMN IF EXISTS team_id;
DROP TABLE IF EXISTS user_team;
DROP TABLE IF EXISTS teams;
ALTER TABLE courses DROP COLUMN IF EXISTS max_team_size;

COMMIT;
