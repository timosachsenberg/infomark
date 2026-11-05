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

package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Team is a fixed group of students within a course who hand in their
// solutions together.
type Team struct {
	ID        int64     `db:"id"`
	CreatedAt time.Time `db:"created_at,omitempty"`
	UpdatedAt time.Time `db:"updated_at,omitempty"`

	CourseID int64  `db:"course_id"`
	JoinCode string `db:"join_code"`
}

// UserTeam links a student to a team.
type UserTeam struct {
	ID       int64 `db:"id"`
	UserID   int64 `db:"user_id"`
	TeamID   int64 `db:"team_id"`
	CourseID int64 `db:"course_id"`
}

// TeamMember is a student belonging to a team.
type TeamMember struct {
	TeamID    int64  `db:"team_id" json:"-"`
	ID        int64  `db:"id" json:"id"`
	FirstName string `db:"first_name" json:"first_name"`
	LastName  string `db:"last_name" json:"last_name"`
	Email     string `db:"email" json:"email"`
}

// TeamMemberList holds team members read from a JSON aggregate column.
type TeamMemberList []TeamMember

// Scan implements sql.Scanner for a JSON array of team members.
func (l *TeamMemberList) Scan(src interface{}) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = TeamMemberList{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into TeamMemberList", src)
	}

	members := TeamMemberList{}
	if err := json.Unmarshal(raw, &members); err != nil {
		return err
	}
	*l = members
	return nil
}
