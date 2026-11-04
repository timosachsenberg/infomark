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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/franela/goblin"
	"github.com/infomark-org/infomark/api/helper"
	"github.com/infomark-org/infomark/configuration"
	"github.com/infomark-org/infomark/email"
	"github.com/infomark-org/infomark/model"
)

func TestTeam(t *testing.T) {
	g := goblin.Goblin(t)
	email.DefaultMail = email.VoidMail

	tape := NewTape()

	var stores *Stores

	adminJWT := tape.NewJWTRequest(1, true)
	tutorJWT := tape.NewJWTRequest(2, false)
	// students 112 to 116 are enrolled in course 1; 112 and 115 are in
	// exercise group 4, 113 in group 5 and 114 in group 8.
	studentJWT := func(id int64) JWTRequest { return tape.NewJWTRequest(id, false) }

	setTeamSize := func(size int) {
		course, err := stores.Course.Get(1)
		g.Assert(err).Equal(nil)
		course.MaxTeamSize = size
		g.Assert(stores.Course.Update(course)).Equal(nil)
	}

	decodeTeam := func(w *httptest.ResponseRecorder) TeamResponse {
		team := TeamResponse{}
		g.Assert(json.NewDecoder(w.Body).Decode(&team)).Equal(nil)
		return team
	}

	memberIDs := func(team TeamResponse) []int64 {
		ids := []int64{}
		for _, m := range team.Members {
			ids = append(ids, m.ID)
		}
		return ids
	}

	// createTeam lets the first student create a team which the others join.
	createTeam := func(founder int64, others ...int64) TeamResponse {
		w := tape.Post("/api/v1/courses/1/teams", H{}, studentJWT(founder))
		g.Assert(w.Code).Equal(http.StatusCreated)
		team := decodeTeam(w)
		for _, id := range others {
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": team.JoinCode}, studentJWT(id))
			g.Assert(w.Code).Equal(http.StatusOK)
		}
		return team
	}

	// newTask creates an open sheet with one task nobody has submitted yet.
	newTask := func() (sheetID int64, taskID int64) {
		sheet, err := stores.Sheet.Create(&model.Sheet{
			Name:      "team sheet",
			PublishAt: NowUTC().Add(-time.Hour),
			DueAt:     NowUTC().Add(time.Hour),
		}, 1)
		g.Assert(err).Equal(nil)
		task, err := stores.Task.Create(&model.Task{Name: "team task", MaxPoints: 10}, sheet.ID)
		g.Assert(err).Equal(nil)
		return sheet.ID, task.ID
	}

	upload := func(taskID int64, user JWTRequest) *httptest.ResponseRecorder {
		filename := fmt.Sprintf("%s/empty.zip", configuration.Configuration.Server.Debugging.Fixtures)
		w, err := tape.Upload(fmt.Sprintf("/api/v1/courses/1/tasks/%d/submission", taskID), filename, "application/zip", user)
		g.Assert(err).Equal(nil)
		return w
	}

	submissionsOfTask := func(taskID int64) []model.Submission {
		p := []model.Submission{}
		g.Assert(tape.DB.Select(&p, "SELECT * FROM submissions WHERE task_id = $1 ORDER BY id", taskID)).Equal(nil)
		return p
	}

	g.Describe("Team", func() {

		g.BeforeEach(func() {
			tape.BeforeEach()
			stores = NewStores(tape.DB)
		})

		g.It("Courses have a maximum team size", func() {
			course, err := stores.Course.Get(1)
			g.Assert(err).Equal(nil)
			g.Assert(course.MaxTeamSize).Equal(1)

			// default for new courses without the field
			w := tape.Post("/api/v1/courses", H{
				"name":                "Teams",
				"description":         "course with teams",
				"begins_at":           NowUTC(),
				"ends_at":             NowUTC().Add(time.Hour * 24 * 30),
				"required_percentage": 50,
			}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusCreated)
			created := CourseResponse{}
			g.Assert(json.NewDecoder(w.Body).Decode(&created)).Equal(nil)
			g.Assert(created.MaxTeamSize).Equal(1)

			w = tape.Post("/api/v1/courses", H{
				"name":                "Teams",
				"description":         "course with teams",
				"begins_at":           NowUTC(),
				"ends_at":             NowUTC().Add(time.Hour * 24 * 30),
				"required_percentage": 50,
				"max_team_size":       3,
			}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusCreated)
			g.Assert(json.NewDecoder(w.Body).Decode(&created)).Equal(nil)
			g.Assert(created.MaxTeamSize).Equal(3)

			w = tape.Post("/api/v1/courses", H{
				"name":                "Teams",
				"description":         "course with teams",
				"begins_at":           NowUTC(),
				"ends_at":             NowUTC().Add(time.Hour * 24 * 30),
				"required_percentage": 50,
				"max_team_size":       0,
			}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
		})

		g.It("Editing a course keeps or changes the team size", func() {
			edit := H{
				"name":                "Info2",
				"description":         "changed",
				"begins_at":           NowUTC(),
				"ends_at":             NowUTC().Add(time.Hour * 24 * 30),
				"required_percentage": 50,
			}

			edit["max_team_size"] = 2
			w := tape.Put("/api/v1/courses/1", edit, adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			course, err := stores.Course.Get(1)
			g.Assert(err).Equal(nil)
			g.Assert(course.MaxTeamSize).Equal(2)

			// omitting the field keeps the value
			delete(edit, "max_team_size")
			w = tape.Put("/api/v1/courses/1", edit, adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			course, err = stores.Course.Get(1)
			g.Assert(err).Equal(nil)
			g.Assert(course.MaxTeamSize).Equal(2)

			// cannot shrink below the largest team
			createTeam(112, 113)
			edit["max_team_size"] = 1
			w = tape.Put("/api/v1/courses/1", edit, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
		})

		g.It("Students cannot form teams in courses without teams", func() {
			w := tape.Post("/api/v1/courses/1/teams", H{}, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusBadRequest)
		})

		g.It("Students create and join teams", func() {
			setTeamSize(2)

			w := tape.Get("/api/v1/courses/1/teams/own", studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusNotFound)

			w = tape.Post("/api/v1/courses/1/teams", H{}, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusCreated)
			team := decodeTeam(w)
			g.Assert(len(team.JoinCode)).Equal(8)
			g.Assert(memberIDs(team)).Equal([]int64{112})
			g.Assert(team.Locked).IsFalse()

			// already in a team
			w = tape.Post("/api/v1/courses/1/teams", H{}, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// unknown code
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": "NOPE0000"}, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusNotFound)

			// codes are case-insensitive
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": " " + team.JoinCode[:4] + " " + team.JoinCode[4:]}, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			g.Assert(len(decodeTeam(w).Members)).Equal(2)

			w = tape.Get("/api/v1/courses/1/teams/own", studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			own := decodeTeam(w)
			g.Assert(own.ID).Equal(team.ID)
			g.Assert(len(own.Members)).Equal(2)

			// the team is full
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": team.JoinCode}, studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// only students are in teams
			w = tape.Post("/api/v1/courses/1/teams", H{}, tutorJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": team.JoinCode}, tutorJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// students cannot create teams for others
			w = tape.Post("/api/v1/courses/1/teams", H{"user_ids": []int64{114, 115}}, studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusForbidden)
		})

		g.It("Students can leave teams without submissions", func() {
			setTeamSize(2)
			team := createTeam(112, 113)

			w := tape.Delete("/api/v1/courses/1/teams/own", studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)

			w = tape.Get("/api/v1/courses/1/teams/own", studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusOK)
			g.Assert(memberIDs(decodeTeam(w))).Equal([]int64{112})

			w = tape.Delete("/api/v1/courses/1/teams/own", studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusNotFound)

			// the last member leaving removes the team
			w = tape.Delete("/api/v1/courses/1/teams/own", studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusOK)
			_, err := stores.Team.Get(team.ID)
			g.Assert(err == nil).IsFalse()
		})

		g.It("Only tutors and admins list all teams", func() {
			setTeamSize(2)
			team := createTeam(112, 113)

			w := tape.Get("/api/v1/courses/1/teams", studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusForbidden)

			w = tape.Get("/api/v1/courses/1/teams", tutorJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			teams := []TeamResponse{}
			g.Assert(json.NewDecoder(w.Body).Decode(&teams)).Equal(nil)
			g.Assert(len(teams)).Equal(1)
			g.Assert(teams[0].ID).Equal(team.ID)
			g.Assert(len(teams[0].Members)).Equal(2)
		})

		g.It("Team members share one submission and its grade", func() {
			setTeamSize(3)
			sheetID, taskID := newTask()
			team := createTeam(112, 113)

			w := upload(taskID, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusOK)

			submissions := submissionsOfTask(taskID)
			g.Assert(len(submissions)).Equal(1)
			submission := submissions[0]
			defer helper.NewSubmissionFileHandle(submission.ID).Delete()
			g.Assert(submission.UserID).Equal(int64(112))
			g.Assert(submission.TeamID.Int64).Equal(team.ID)

			// the team mate sees the submission and its result
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/tasks/%d/submission", taskID), studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/submissions/%d/file", submission.ID), studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/tasks/%d/result", taskID), studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			result := GradeResponse{}
			g.Assert(json.NewDecoder(w.Body).Decode(&result)).Equal(nil)
			g.Assert(result.SubmissionID).Equal(submission.ID)

			// other students do not
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/tasks/%d/submission", taskID), studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusNotFound)
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/submissions/%d/file", submission.ID), studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusForbidden)

			// an upload of the team mate replaces the team submission
			w = upload(taskID, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			submissions = submissionsOfTask(taskID)
			g.Assert(len(submissions)).Equal(1)
			g.Assert(submissions[0].ID).Equal(submission.ID)
			g.Assert(submissions[0].UserID).Equal(int64(112))

			// the team is locked now
			w = tape.Get("/api/v1/courses/1/teams/own", studentJWT(112))
			g.Assert(decodeTeam(w).Locked).IsTrue()
			w = tape.Delete("/api/v1/courses/1/teams/own", studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusBadRequest)
			w = tape.Post("/api/v1/courses/1/teams/join", H{"join_code": team.JoinCode}, studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// grading the team submission once gives points to all members
			grade, err := stores.Grade.GetForSubmission(submission.ID)
			g.Assert(err).Equal(nil)
			w = tape.Put(fmt.Sprintf("/api/v1/courses/1/grades/%d", grade.ID), H{
				"acquired_points": 7,
				"feedback":        "well done, team",
			}, tutorJWT)
			g.Assert(w.Code).Equal(http.StatusOK)

			for _, id := range []int64{112, 113} {
				points, err := stores.Sheet.PointsForUser(id, sheetID)
				g.Assert(err).Equal(nil)
				g.Assert(len(points)).Equal(1)
				g.Assert(points[0].AquiredPoints).Equal(7)

				coursePoints, err := stores.Course.PointsForUser(id, 1)
				g.Assert(err).Equal(nil)
				found := false
				for _, p := range coursePoints {
					if int64(p.SheetID) == sheetID {
						found = true
						g.Assert(p.AquiredPoints).Equal(7)
					}
				}
				g.Assert(found).IsTrue()
			}
			points, err := stores.Sheet.PointsForUser(114, sheetID)
			g.Assert(err).Equal(nil)
			g.Assert(len(points)).Equal(0)

			// the summary lists the team points for every member
			overview, err := stores.Grade.GetOverviewGrades(1, 0)
			g.Assert(err).Equal(nil)
			credited := map[int64]int{}
			for _, entry := range overview {
				if entry.SheetID == sheetID {
					credited[entry.UserID] = entry.Points
				}
			}
			g.Assert(credited).Equal(map[int64]int{112: 7, 113: 7})

			// tutors of both exercise groups see the submission with its team
			for _, groupID := range []int64{4, 5} {
				w = tape.Get(fmt.Sprintf("/api/v1/courses/1/grades?group_id=%d&task_id=%d", groupID, taskID), tutorJWT)
				g.Assert(w.Code).Equal(http.StatusOK)
				grades := []GradeResponse{}
				g.Assert(json.NewDecoder(w.Body).Decode(&grades)).Equal(nil)
				g.Assert(len(grades)).Equal(1)
				g.Assert(grades[0].TeamID.Int64).Equal(team.ID)
				g.Assert(len(grades[0].TeamMembers)).Equal(2)
			}
			w = tape.Get(fmt.Sprintf("/api/v1/courses/1/grades?group_id=8&task_id=%d", taskID), tutorJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			grades := []GradeResponse{}
			g.Assert(json.NewDecoder(w.Body).Decode(&grades)).Equal(nil)
			g.Assert(len(grades)).Equal(0)

			// the team submission is not missing for anybody in the team
			missing, err := stores.Task.GetAllMissingTasksForUser(113)
			g.Assert(err).Equal(nil)
			for _, m := range missing {
				g.Assert(m.ID == taskID).IsFalse()
			}
		})

		g.It("An earlier own submission becomes the team submission", func() {
			setTeamSize(2)
			// students 112 and 113 handed in task 1 on their own (submissions 3001 and 3031)
			team := createTeam(112, 113)
			defer helper.NewSubmissionFileHandle(3001).Delete()

			sheet, err := stores.Task.IdentifySheetOfTask(1)
			g.Assert(err).Equal(nil)
			sheet.PublishAt = NowUTC().Add(-time.Hour)
			sheet.DueAt = NowUTC().Add(time.Hour)
			g.Assert(stores.Sheet.Update(sheet)).Equal(nil)

			w := upload(1, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusOK)

			submission, err := stores.Submission.Get(3001)
			g.Assert(err).Equal(nil)
			g.Assert(submission.TeamID.Int64).Equal(team.ID)

			// the team submission counts for 113 instead of the own one
			counted, err := stores.Submission.GetByUserAndTask(113, 1)
			g.Assert(err).Equal(nil)
			g.Assert(counted.ID).Equal(int64(3001))

			g.Assert(len(submissionsOfTask(1))).Equal(300)

			// the tutor of 113 only sees the submission which counts
			w = tape.Get("/api/v1/courses/1/grades?group_id=5&task_id=1&user_id=113", tutorJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			grades := []GradeResponse{}
			g.Assert(json.NewDecoder(w.Body).Decode(&grades)).Equal(nil)
			g.Assert(len(grades)).Equal(1)
			g.Assert(grades[0].SubmissionID).Equal(int64(3001))

			submissions, err := stores.Submission.GetFiltered(1, 5, 113, 0, 1)
			g.Assert(err).Equal(nil)
			g.Assert(len(submissions)).Equal(1)
			g.Assert(submissions[0].ID).Equal(int64(3001))
		})

		g.It("Course admins manage teams", func() {
			setTeamSize(2)

			w := tape.Post("/api/v1/courses/1/teams", H{"user_ids": []int64{114, 115}}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusCreated)
			team := decodeTeam(w)
			g.Assert(len(team.Members)).Equal(2)

			// students and tutors cannot manage teams
			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d", team.ID), studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusForbidden)
			w = tape.Post(fmt.Sprintf("/api/v1/courses/1/teams/%d/members", team.ID), H{"user_id": 116}, tutorJWT)
			g.Assert(w.Code).Equal(http.StatusForbidden)

			// invalid team compositions
			w = tape.Post("/api/v1/courses/1/teams", H{"user_ids": []int64{116, 117, 118}}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
			w = tape.Post("/api/v1/courses/1/teams", H{"user_ids": []int64{116, 114}}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
			w = tape.Post("/api/v1/courses/1/teams", H{"user_ids": []int64{116, 2}}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// the team is full
			w = tape.Post(fmt.Sprintf("/api/v1/courses/1/teams/%d/members", team.ID), H{"user_id": 116}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			setTeamSize(3)
			w = tape.Post(fmt.Sprintf("/api/v1/courses/1/teams/%d/members", team.ID), H{"user_id": 116}, adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			g.Assert(memberIDs(decodeTeam(w))).Equal(memberIDs(decodeTeam(tape.Get("/api/v1/courses/1/teams/own", studentJWT(116)))))

			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d/members/116", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d/members/116", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusNotFound)

			// teams of other courses are not reachable
			w = tape.Delete(fmt.Sprintf("/api/v1/courses/2/teams/%d", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusNotFound)

			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)
			w = tape.Get("/api/v1/courses/1/teams/own", studentJWT(114))
			g.Assert(w.Code).Equal(http.StatusNotFound)
		})

		g.It("Deleting a team keeps its submissions with the uploader", func() {
			setTeamSize(2)
			sheetID, taskID := newTask()
			team := createTeam(112, 113)

			w := upload(taskID, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			submission := submissionsOfTask(taskID)[0]
			defer helper.NewSubmissionFileHandle(submission.ID).Delete()

			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)

			submission = submissionsOfTask(taskID)[0]
			g.Assert(submission.TeamID.Valid).IsFalse()
			g.Assert(submission.UserID).Equal(int64(113))

			points, err := stores.Sheet.PointsForUser(113, sheetID)
			g.Assert(err).Equal(nil)
			g.Assert(len(points)).Equal(1)
			points, err = stores.Sheet.PointsForUser(112, sheetID)
			g.Assert(err).Equal(nil)
			g.Assert(len(points)).Equal(0)
		})

		g.It("Former members cannot overwrite the submission of their old team", func() {
			setTeamSize(2)
			_, taskID := newTask()
			team := createTeam(112, 113)

			w := upload(taskID, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusOK)
			submission := submissionsOfTask(taskID)[0]
			defer helper.NewSubmissionFileHandle(submission.ID).Delete()

			w = tape.Delete(fmt.Sprintf("/api/v1/courses/1/teams/%d/members/113", team.ID), adminJWT)
			g.Assert(w.Code).Equal(http.StatusOK)

			w = upload(taskID, studentJWT(113))
			g.Assert(w.Code).Equal(http.StatusBadRequest)

			// the remaining member still works on the team submission
			w = upload(taskID, studentJWT(112))
			g.Assert(w.Code).Equal(http.StatusOK)
			g.Assert(len(submissionsOfTask(taskID))).Equal(1)
		})

		g.AfterEach(func() {
			tape.AfterEach()
		})
	})
}
