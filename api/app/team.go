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
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/infomark-org/infomark/auth/authenticate"
	"github.com/infomark-org/infomark/auth/authorize"
	"github.com/infomark-org/infomark/database"
	"github.com/infomark-org/infomark/model"
	"github.com/infomark-org/infomark/symbol"
)

var (
	errNoTeams       = errors.New("this course does not use teams")
	errOnlyStudents  = errors.New("only students can be members of a team")
	errAlreadyInTeam = errors.New("the student is already in a team in this course")
	errTeamLocked    = errors.New("the team has already handed in solutions; only a course admin can change it")
)

// joinCodeAlphabet avoids characters which are easily confused (0/O, 1/I).
const joinCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const joinCodeLength = 8

func newJoinCode() (string, error) {
	code := make([]byte, joinCodeLength)
	max := big.NewInt(int64(len(joinCodeAlphabet)))
	for k := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[k] = joinCodeAlphabet[n.Int64()]
	}
	return string(code), nil
}

// TeamResource specifies team management handler.
type TeamResource struct {
	Stores *Stores
}

// NewTeamResource create and returns a TeamResource.
func NewTeamResource(stores *Stores) *TeamResource {
	return &TeamResource{
		Stores: stores,
	}
}

// isStudent reports whether a user is enrolled as a student in a course.
func (rs *TeamResource) isStudent(userID int64, courseID int64) (bool, error) {
	role, err := rs.Stores.Course.RoleInCourse(userID, courseID)
	if err != nil {
		return false, err
	}
	return role == authorize.STUDENT, nil
}

// teamOf returns the team of a student in a course or nil.
func (rs *TeamResource) teamOf(userID int64, courseID int64) (*model.Team, error) {
	team, err := rs.Stores.Team.GetOfUserInCourse(userID, courseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return team, err
}

// createTeam creates an empty team with a fresh join code.
func (rs *TeamResource) createTeam(courseID int64) (*model.Team, error) {
	var lastErr error
	// join codes are random; retry in the unlikely case of a collision
	for attempt := 0; attempt < 3; attempt++ {
		code, err := newJoinCode()
		if err != nil {
			return nil, err
		}
		team, err := rs.Stores.Team.Create(&model.Team{CourseID: courseID, JoinCode: code})
		if err == nil {
			return team, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// renderTeam renders a team together with its members.
func (rs *TeamResource) renderTeam(w http.ResponseWriter, r *http.Request, team *model.Team, status int) {
	members, err := rs.Stores.Team.Members(team.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	locked, err := rs.Stores.Team.HasSubmissions(team.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	render.Status(r, status)
	if err := render.Render(w, r, newTeamResponse(team, members, locked)); err != nil {
		render.Render(w, r, ErrRender(err))
	}
}

// IndexHandler is public endpoint for
// URL: /courses/{course_id}/teams
// URLPARAM: course_id,integer
// METHOD: get
// TAG: teams
// RESPONSE: 200,TeamResponseList
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// SUMMARY:  list all teams of a course with their members
func (rs *TeamResource) IndexHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)

	teams, err := rs.Stores.Team.TeamsOfCourse(course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	members, err := rs.Stores.Team.MembersInCourse(course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	lockedIDs, err := rs.Stores.Team.TeamIDsWithSubmissions(course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	if err := render.RenderList(w, r, newTeamListResponse(teams, members, lockedIDs)); err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}

	render.Status(r, http.StatusOK)
}

// GetMineHandler is public endpoint for
// URL: /courses/{course_id}/teams/own
// URLPARAM: course_id,integer
// METHOD: get
// TAG: teams
// RESPONSE: 200,TeamResponse
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// RESPONSE: 404,NotFound
// SUMMARY:  get the team of the request identity
func (rs *TeamResource) GetMineHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)

	team, err := rs.teamOf(accessClaims.LoginID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if team == nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	rs.renderTeam(w, r, team, http.StatusOK)
}

// CreateHandler is public endpoint for
// URL: /courses/{course_id}/teams
// URLPARAM: course_id,integer
// METHOD: post
// TAG: teams
// REQUEST: TeamCreateRequest
// RESPONSE: 201,TeamResponse
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// SUMMARY:  create a team; students create their own team, course admins pass the members
func (rs *TeamResource) CreateHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	givenRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	// students may send an empty body
	data := &TeamCreateRequest{}
	if err := render.Bind(r, data); err != nil && !errors.Is(err, io.EOF) {
		render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	if course.MaxTeamSize < 2 {
		render.Render(w, r, ErrBadRequestWithDetails(errNoTeams))
		return
	}

	memberIDs := data.UserIDs
	switch givenRole {
	case authorize.STUDENT:
		if len(memberIDs) > 0 {
			render.Render(w, r, ErrUnauthorizedWithDetails(errors.New("students can only create a team for themselves")))
			return
		}
		memberIDs = []int64{accessClaims.LoginID}
	case authorize.ADMIN:
		if len(memberIDs) == 0 {
			render.Render(w, r, ErrBadRequestWithDetails(errors.New("user_ids is required")))
			return
		}
	default:
		render.Render(w, r, ErrBadRequestWithDetails(errOnlyStudents))
		return
	}

	if len(memberIDs) > course.MaxTeamSize {
		render.Render(w, r, ErrBadRequestWithDetails(fmt.Errorf("a team has at most %d members", course.MaxTeamSize)))
		return
	}

	seen := map[int64]bool{}
	for _, userID := range memberIDs {
		if seen[userID] {
			render.Render(w, r, ErrBadRequestWithDetails(fmt.Errorf("user %d is listed twice", userID)))
			return
		}
		seen[userID] = true

		student, err := rs.isStudent(userID, course.ID)
		if err != nil {
			render.Render(w, r, ErrInternalServerErrorWithDetails(err))
			return
		}
		if !student {
			render.Render(w, r, ErrBadRequestWithDetails(fmt.Errorf("user %d is not a student in this course", userID)))
			return
		}
		team, err := rs.teamOf(userID, course.ID)
		if err != nil {
			render.Render(w, r, ErrInternalServerErrorWithDetails(err))
			return
		}
		if team != nil {
			render.Render(w, r, ErrBadRequestWithDetails(fmt.Errorf("user %d is already in a team in this course", userID)))
			return
		}
	}

	team, err := rs.createTeam(course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	for _, userID := range memberIDs {
		if err := rs.Stores.Team.AddMember(team.ID, userID, course.MaxTeamSize); err != nil {
			// do not leave a half-built team behind
			rs.Stores.Team.Delete(team.ID)
			render.Render(w, r, ErrInternalServerErrorWithDetails(err))
			return
		}
	}

	rs.renderTeam(w, r, team, http.StatusCreated)
}

// JoinHandler is public endpoint for
// URL: /courses/{course_id}/teams/join
// URLPARAM: course_id,integer
// METHOD: post
// TAG: teams
// REQUEST: TeamJoinRequest
// RESPONSE: 200,TeamResponse
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// RESPONSE: 404,NotFound
// SUMMARY:  join a team using its join code
func (rs *TeamResource) JoinHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	givenRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	data := &TeamJoinRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	if course.MaxTeamSize < 2 {
		render.Render(w, r, ErrBadRequestWithDetails(errNoTeams))
		return
	}
	if givenRole != authorize.STUDENT {
		render.Render(w, r, ErrBadRequestWithDetails(errOnlyStudents))
		return
	}

	current, err := rs.teamOf(accessClaims.LoginID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if current != nil {
		render.Render(w, r, ErrBadRequestWithDetails(errors.New("you are already in a team; leave it first")))
		return
	}

	team, err := rs.Stores.Team.GetByJoinCode(course.ID, data.JoinCode)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	locked, err := rs.Stores.Team.HasSubmissions(team.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if locked {
		render.Render(w, r, ErrBadRequestWithDetails(errTeamLocked))
		return
	}

	if err := rs.Stores.Team.AddMember(team.ID, accessClaims.LoginID, course.MaxTeamSize); err != nil {
		if errors.Is(err, database.ErrTeamFull) {
			render.Render(w, r, ErrBadRequestWithDetails(err))
			return
		}
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	rs.renderTeam(w, r, team, http.StatusOK)
}

// LeaveHandler is public endpoint for
// URL: /courses/{course_id}/teams/own
// URLPARAM: course_id,integer
// METHOD: delete
// TAG: teams
// RESPONSE: 204,NoContent
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// RESPONSE: 404,NotFound
// SUMMARY:  leave the team of the request identity as long as it has not handed in solutions
func (rs *TeamResource) LeaveHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)

	team, err := rs.teamOf(accessClaims.LoginID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if team == nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	locked, err := rs.Stores.Team.HasSubmissions(team.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if locked {
		render.Render(w, r, ErrBadRequestWithDetails(errTeamLocked))
		return
	}

	if err := rs.Stores.Team.RemoveMember(team.ID, accessClaims.LoginID); err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

// DeleteHandler is public endpoint for
// URL: /courses/{course_id}/teams/{team_id}
// URLPARAM: course_id,integer
// URLPARAM: team_id,integer
// METHOD: delete
// TAG: teams
// RESPONSE: 204,NoContent
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// SUMMARY:  delete a team; its submissions stay with the students who uploaded them
func (rs *TeamResource) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	team := r.Context().Value(symbol.CtxKeyTeam).(*model.Team)

	if err := rs.Stores.Team.Delete(team.ID); err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

// AddMemberHandler is public endpoint for
// URL: /courses/{course_id}/teams/{team_id}/members
// URLPARAM: course_id,integer
// URLPARAM: team_id,integer
// METHOD: post
// TAG: teams
// REQUEST: TeamMemberRequest
// RESPONSE: 200,TeamResponse
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// SUMMARY:  add a student to a team
func (rs *TeamResource) AddMemberHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	team := r.Context().Value(symbol.CtxKeyTeam).(*model.Team)

	data := &TeamMemberRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	student, err := rs.isStudent(data.UserID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if !student {
		render.Render(w, r, ErrBadRequestWithDetails(errOnlyStudents))
		return
	}

	current, err := rs.teamOf(data.UserID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if current != nil {
		render.Render(w, r, ErrBadRequestWithDetails(errAlreadyInTeam))
		return
	}

	if err := rs.Stores.Team.AddMember(team.ID, data.UserID, course.MaxTeamSize); err != nil {
		if errors.Is(err, database.ErrTeamFull) {
			render.Render(w, r, ErrBadRequestWithDetails(err))
			return
		}
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	rs.renderTeam(w, r, team, http.StatusOK)
}

// RemoveMemberHandler is public endpoint for
// URL: /courses/{course_id}/teams/{team_id}/members/{user_id}
// URLPARAM: course_id,integer
// URLPARAM: team_id,integer
// URLPARAM: user_id,integer
// METHOD: delete
// TAG: teams
// RESPONSE: 204,NoContent
// RESPONSE: 400,BadRequest
// RESPONSE: 401,Unauthenticated
// RESPONSE: 403,Unauthorized
// RESPONSE: 404,NotFound
// SUMMARY:  remove a student from a team; an empty team is deleted
func (rs *TeamResource) RemoveMemberHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	team := r.Context().Value(symbol.CtxKeyTeam).(*model.Team)

	userID, err := strconv.ParseInt(chi.URLParam(r, "user_id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	current, err := rs.teamOf(userID, course.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if current == nil || current.ID != team.ID {
		render.Render(w, r, ErrNotFound)
		return
	}

	if err := rs.Stores.Team.RemoveMember(team.ID, userID); err != nil {
		render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

// .............................................................................

// Context middleware is used to load a Team object from the URL parameter
// `team_id`. In case the team could not be found in the course, we stop here
// and return a 404. We do NOT check whether the identity is authorized to get
// this team.
func (rs *TeamResource) Context(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)

		teamID, err := strconv.ParseInt(chi.URLParam(r, "team_id"), 10, 64)
		if err != nil {
			render.Render(w, r, ErrNotFound)
			return
		}

		team, err := rs.Stores.Team.Get(teamID)
		if err != nil || team.CourseID != course.ID {
			render.Render(w, r, ErrNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), symbol.CtxKeyTeam, team)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
