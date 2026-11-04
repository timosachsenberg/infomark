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
	"net/http"

	"github.com/go-chi/render"
	"github.com/infomark-org/infomark/model"
)

// TeamMemberResponse is a student in a team.
type TeamMemberResponse struct {
	ID        int64  `json:"id" example:"112"`
	FirstName string `json:"first_name" example:"Max"`
	LastName  string `json:"last_name" example:"Mustermensch"`
	Email     string `json:"email" example:"max@uni-tuebingen.de"`
}

func newTeamMemberResponses(members []model.TeamMember) []TeamMemberResponse {
	list := []TeamMemberResponse{}
	for _, m := range members {
		list = append(list, TeamMemberResponse{
			ID:        m.ID,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			Email:     m.Email,
		})
	}
	return list
}

// TeamResponse is the response payload for team management.
type TeamResponse struct {
	ID       int64  `json:"id" example:"3"`
	JoinCode string `json:"join_code" example:"K7Q2M9XD"`
	// Locked teams handed in solutions; only course admins can change them.
	Locked  bool                 `json:"locked" example:"false"`
	Members []TeamMemberResponse `json:"members" example:""`
}

// Render post-processes a TeamResponse.
func (body *TeamResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func newTeamResponse(team *model.Team, members []model.TeamMember, locked bool) *TeamResponse {
	return &TeamResponse{
		ID:       team.ID,
		JoinCode: team.JoinCode,
		Locked:   locked,
		Members:  newTeamMemberResponses(members),
	}
}

func newTeamListResponse(teams []model.Team, members []model.TeamMember, lockedIDs []int64) []render.Renderer {
	membersOf := map[int64][]model.TeamMember{}
	for _, m := range members {
		membersOf[m.TeamID] = append(membersOf[m.TeamID], m)
	}
	locked := map[int64]bool{}
	for _, id := range lockedIDs {
		locked[id] = true
	}

	list := []render.Renderer{}
	for k := range teams {
		list = append(list, newTeamResponse(&teams[k], membersOf[teams[k].ID], locked[teams[k].ID]))
	}
	return list
}
