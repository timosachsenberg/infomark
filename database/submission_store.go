// InfoMark - a platform for managing courses with
//            distributing exercise sheets and testing exercise submissions
// Copyright (C) 2019 ComputerGraphics Tuebingen
//               2020-present InfoMark.org
// Authors: Patrick Wieschollek
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package database

import (
	"fmt"

	"github.com/infomark-org/infomark/model"
	"github.com/jmoiron/sqlx"
)

// CountedSubmission returns an SQL condition on the submission with the given
// table alias which drops own submissions that no longer count because the
// uploader's team handed in for the same task. Lists for tutors use it so that
// only submissions which earn points show up.
func CountedSubmission(alias string) string {
	return fmt.Sprintf(`NOT (
  %[1]s.team_id IS NULL
  AND EXISTS (
    SELECT 1
    FROM user_team cut
    INNER JOIN submissions cts ON cts.team_id = cut.team_id
    WHERE cut.user_id = %[1]s.user_id AND cts.task_id = %[1]s.task_id
  )
)`, alias)
}

type SubmissionStore struct {
	db *sqlx.DB
}

func NewSubmissionStore(db *sqlx.DB) *SubmissionStore {
	return &SubmissionStore{
		db: db,
	}
}

func (s *SubmissionStore) Get(submissionID int64) (*model.Submission, error) {
	p := model.Submission{ID: submissionID}
	err := s.db.Get(&p, `SELECT * FROM submissions WHERE id = $1 LIMIT 1;`, p.ID)
	return &p, err
}

// GetByUserAndTask returns the submission which counts for a user and a task:
// the submission of the user's team if there is one, otherwise the user's own.
func (s *SubmissionStore) GetByUserAndTask(userID int64, taskID int64) (*model.Submission, error) {
	p := model.Submission{}
	err := s.db.Get(&p, `
SELECT
  s.*
FROM
  submissions s
INNER JOIN effective_submissions es ON es.submission_id = s.id
WHERE
  es.user_id = $1
AND
  es.task_id = $2
LIMIT 1;`,
		userID, taskID)
	return &p, err
}

// GetUploadedByUserAndTask returns the submission a user uploaded for a task,
// regardless of any team.
func (s *SubmissionStore) GetUploadedByUserAndTask(userID int64, taskID int64) (*model.Submission, error) {
	p := model.Submission{}
	err := s.db.Get(&p, `
SELECT
  *
FROM
  submissions
WHERE
  user_id = $1
AND
  task_id = $2
LIMIT 1;`,
		userID, taskID)
	return &p, err
}

// GetByTeamAndTask returns the submission of a team for a task.
func (s *SubmissionStore) GetByTeamAndTask(teamID int64, taskID int64) (*model.Submission, error) {
	p := model.Submission{}
	err := s.db.Get(&p, `
SELECT
  *
FROM
  submissions
WHERE
  team_id = $1
AND
  task_id = $2
LIMIT 1;`,
		teamID, taskID)
	return &p, err
}

// SetTeam moves a submission into a team.
func (s *SubmissionStore) SetTeam(submissionID int64, teamID int64) error {
	_, err := s.db.Exec(`UPDATE submissions SET team_id = $2 WHERE id = $1;`, submissionID, teamID)
	return err
}

// IsParticipant reports whether a submission is credited to a user, i.e. the
// user uploaded it or is a member of the team it belongs to.
func (s *SubmissionStore) IsParticipant(submissionID int64, userID int64) (bool, error) {
	var exists bool
	err := s.db.Get(&exists, `
SELECT EXISTS(
  SELECT 1 FROM submission_participants WHERE submission_id = $1 AND user_id = $2
);`, submissionID, userID)
	return exists, err
}

func (s *SubmissionStore) Create(p *model.Submission) (*model.Submission, error) {
	newID, err := Insert(s.db, "submissions", p)
	if err != nil {
		return nil, err
	}
	return s.Get(newID)
}

func (s *SubmissionStore) GetFiltered(filterCourseID, filterGroupID, filterUserID, filterSheetID, filterTaskID int64) ([]model.Submission, error) {

	p := []model.Submission{}
	err := s.db.Select(&p,
		`
SELECT DISTINCT
  s.*
FROM
  submissions s
INNER JOIN submission_participants sp ON sp.submission_id = s.id
INNER JOIN user_group ug ON ug.user_id = sp.user_id
INNER JOIN groups g ON g.id = ug.group_id
INNEr JOIN task_sheet ts ON ts.task_id = s.task_id
WHERE
  ($1 = 0 or sp.user_id = $1)
AND
  ($2 = 0 or s.task_id = $2)
AND
  ($3 = 0 or ug.group_id = $3)
AND
  ($4 = 0 or ts.sheet_id = $4)
AND
  ($5 = 0 or g.course_id = $5)
AND
  `+CountedSubmission("s")+`
ORDER BY
  s.id
`,
		filterUserID, filterTaskID, filterGroupID, filterSheetID, filterCourseID)
	return p, err
}
