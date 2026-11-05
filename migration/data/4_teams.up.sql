BEGIN;

-- Maximum number of students per team. 1 means individual work (no teams).
ALTER TABLE courses ADD COLUMN max_team_size INT NOT NULL DEFAULT 1;

-- Fixed teams of students within a course.
CREATE TABLE teams (
  id SERIAL NOT NULL PRIMARY KEY,
  created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
  updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp,

  course_id INT NOT NULL,
  -- shared by team members so that classmates can join the team
  join_code TEXT NOT NULL UNIQUE,

  UNIQUE (id, course_id),
  FOREIGN KEY (course_id) REFERENCES courses (id) ON DELETE CASCADE
);

CREATE TABLE user_team (
  id SERIAL NOT NULL PRIMARY KEY,
  user_id INT NOT NULL,
  team_id INT NOT NULL,
  course_id INT NOT NULL,

  -- a student belongs to at most one team per course
  UNIQUE (user_id, course_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  FOREIGN KEY (team_id, course_id) REFERENCES teams (id, course_id) ON DELETE CASCADE
);

-- A submission uploaded by a team member belongs to the whole team. The
-- uploader stays in submissions.user_id. If a team is deleted, its
-- submissions fall back to their uploader.
ALTER TABLE submissions ADD COLUMN team_id INT NULL REFERENCES teams (id) ON DELETE SET NULL;

-- A team has at most one submission per task.
CREATE UNIQUE INDEX submissions_task_id_team_id_key ON submissions (task_id, team_id) WHERE team_id IS NOT NULL;
CREATE INDEX submissions_team_id_idx ON submissions (team_id) WHERE team_id IS NOT NULL;
CREATE INDEX submissions_user_id_idx ON submissions (user_id);

-- Everybody a submission is credited to: its uploader and, for team
-- submissions, every current member of the team.
CREATE VIEW submission_participants AS
SELECT s.id AS submission_id, s.user_id, s.task_id, s.team_id
FROM submissions s
UNION
SELECT s.id AS submission_id, ut.user_id, s.task_id, s.team_id
FROM submissions s
INNER JOIN user_team ut ON ut.team_id = s.team_id;

-- The one submission that counts for a user and a task: a team submission
-- if there is one, otherwise the user's own (newest first as a tie-breaker).
CREATE VIEW effective_submissions AS
SELECT DISTINCT ON (user_id, task_id)
  user_id,
  task_id,
  submission_id
FROM submission_participants
ORDER BY user_id, task_id, (team_id IS NULL), submission_id DESC;

COMMIT;
