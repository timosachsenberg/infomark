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

package app

import (
	"errors"
	"net/http"
	"strings"
)

// TeamCreateRequest is the request payload to create a team. Students create
// a team for themselves and send an empty body; course admins list the
// students who form the team.
type TeamCreateRequest struct {
	UserIDs []int64 `json:"user_ids" example:"[112, 113]"`
}

// Bind preprocesses a TeamCreateRequest.
func (body *TeamCreateRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing \"team\" data")
	}
	return nil
}

// TeamJoinRequest is the request payload to join a team.
type TeamJoinRequest struct {
	JoinCode string `json:"join_code" example:"K7Q2M9XD"`
}

// Bind preprocesses a TeamJoinRequest.
func (body *TeamJoinRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing \"join_code\" data")
	}
	body.JoinCode = normalizeJoinCode(body.JoinCode)
	if body.JoinCode == "" {
		return errors.New("join_code is required")
	}
	return nil
}

// TeamMemberRequest is the request payload to add a student to a team.
type TeamMemberRequest struct {
	UserID int64 `json:"user_id" example:"112"`
}

// Bind preprocesses a TeamMemberRequest.
func (body *TeamMemberRequest) Bind(r *http.Request) error {
	if body == nil || body.UserID <= 0 {
		return errors.New("user_id is required")
	}
	return nil
}

func normalizeJoinCode(code string) string {
	return strings.ToUpper(strings.Join(strings.Fields(code), ""))
}
