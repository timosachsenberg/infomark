// InfoMark - a platform for managing courses with
//            distributing exercise sheets and testing exercise submissions
// Copyright (C) 2020-present InfoMark.org
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
	"errors"

	"github.com/infomark-org/infomark/model"
	"github.com/jmoiron/sqlx"
)

// ErrTeamFull is returned when a team already has the maximum number of members.
var ErrTeamFull = errors.New("the team is already full")

type TeamStore struct {
	db *sqlx.DB
}

func NewTeamStore(db *sqlx.DB) *TeamStore {
	return &TeamStore{
		db: db,
	}
}

func (s *TeamStore) Get(teamID int64) (*model.Team, error) {
	p := model.Team{}
	err := s.db.Get(&p, `SELECT * FROM teams WHERE id = $1 LIMIT 1;`, teamID)
	return &p, err
}

// GetByJoinCode finds the team in a course which uses the given join code.
func (s *TeamStore) GetByJoinCode(courseID int64, joinCode string) (*model.Team, error) {
	p := model.Team{}
	err := s.db.Get(&p, `
SELECT
  *
FROM
  teams
WHERE
  course_id = $1
AND
  join_code = $2
LIMIT 1;`, courseID, joinCode)
	return &p, err
}

// GetOfUserInCourse returns the team of a student in a course.
func (s *TeamStore) GetOfUserInCourse(userID int64, courseID int64) (*model.Team, error) {
	p := model.Team{}
	err := s.db.Get(&p, `
SELECT
  t.*
FROM
  teams t
INNER JOIN user_team ut ON ut.team_id = t.id
WHERE
  ut.user_id = $1
AND
  ut.course_id = $2
LIMIT 1;`, userID, courseID)
	return &p, err
}

// TeamsOfCourse returns all teams of a course.
func (s *TeamStore) TeamsOfCourse(courseID int64) ([]model.Team, error) {
	p := []model.Team{}
	err := s.db.Select(&p, `
SELECT
  *
FROM
  teams
WHERE
  course_id = $1
ORDER BY
  id;`, courseID)
	return p, err
}

// Members returns all students in a team.
func (s *TeamStore) Members(teamID int64) ([]model.TeamMember, error) {
	p := []model.TeamMember{}
	err := s.db.Select(&p, `
SELECT
  ut.team_id,
  u.id,
  u.first_name,
  u.last_name,
  u.email
FROM
  user_team ut
INNER JOIN users u ON u.id = ut.user_id
WHERE
  ut.team_id = $1
ORDER BY
  u.last_name, u.first_name;`, teamID)
	return p, err
}

// MembersInCourse returns the members of all teams in a course.
func (s *TeamStore) MembersInCourse(courseID int64) ([]model.TeamMember, error) {
	p := []model.TeamMember{}
	err := s.db.Select(&p, `
SELECT
  ut.team_id,
  u.id,
  u.first_name,
  u.last_name,
  u.email
FROM
  user_team ut
INNER JOIN users u ON u.id = ut.user_id
WHERE
  ut.course_id = $1
ORDER BY
  ut.team_id, u.last_name, u.first_name;`, courseID)
	return p, err
}

// TeamIDsWithSubmissions returns the ids of all teams in a course which
// already handed in a solution. Their membership is locked for students.
func (s *TeamStore) TeamIDsWithSubmissions(courseID int64) ([]int64, error) {
	p := []int64{}
	err := s.db.Select(&p, `
SELECT DISTINCT
  t.id
FROM
  teams t
INNER JOIN submissions s ON s.team_id = t.id
WHERE
  t.course_id = $1;`, courseID)
	return p, err
}

// HasSubmissions reports whether a team already handed in a solution.
func (s *TeamStore) HasSubmissions(teamID int64) (bool, error) {
	var exists bool
	err := s.db.Get(&exists, `SELECT EXISTS(SELECT 1 FROM submissions WHERE team_id = $1);`, teamID)
	return exists, err
}

// LargestTeamSize returns the number of members of the largest team in a course.
func (s *TeamStore) LargestTeamSize(courseID int64) (int, error) {
	var size int
	err := s.db.Get(&size, `
SELECT
  COALESCE(MAX(members), 0)
FROM (
  SELECT COUNT(*) AS members FROM user_team WHERE course_id = $1 GROUP BY team_id
) sizes;`, courseID)
	return size, err
}

func (s *TeamStore) Create(p *model.Team) (*model.Team, error) {
	newID, err := Insert(s.db, "teams", p)
	if err != nil {
		return nil, err
	}
	return s.Get(newID)
}

func (s *TeamStore) Delete(teamID int64) error {
	return Delete(s.db, "teams", teamID)
}

// AddMember adds a student to a team unless the team already has maxSize
// members. The team row is locked so that concurrent joins cannot overfill it.
func (s *TeamStore) AddMember(teamID int64, userID int64, maxSize int) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var courseID int64
	if err := tx.Get(&courseID, `SELECT course_id FROM teams WHERE id = $1 FOR UPDATE;`, teamID); err != nil {
		return err
	}

	var members int
	if err := tx.Get(&members, `SELECT COUNT(*) FROM user_team WHERE team_id = $1;`, teamID); err != nil {
		return err
	}
	if members >= maxSize {
		return ErrTeamFull
	}

	if _, err := tx.Exec(`
INSERT INTO
  user_team (id, user_id, team_id, course_id)
VALUES (DEFAULT, $1, $2, $3);`, userID, teamID, courseID); err != nil {
		return err
	}

	return tx.Commit()
}

// RemoveMember removes a student from a team. A team without members is
// deleted.
func (s *TeamStore) RemoveMember(teamID int64, userID int64) error {
	if _, err := s.db.Exec(`DELETE FROM user_team WHERE team_id = $1 AND user_id = $2;`, teamID, userID); err != nil {
		return err
	}

	_, err := s.db.Exec(`
DELETE FROM
  teams
WHERE
  id = $1
AND
  NOT EXISTS (SELECT 1 FROM user_team WHERE team_id = $1);`, teamID)
	return err
}
