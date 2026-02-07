package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type SubmissionHandler struct {
	cfg *config.Config
}

func NewSubmissionHandler(cfg *config.Config) *SubmissionHandler {
	return &SubmissionHandler{cfg: cfg}
}

func (h *SubmissionHandler) Accept(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	assignmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid assignment id")
	}

	var assignment models.Assignment
	if err := database.DB.Preload("Course").First(&assignment, assignmentID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "assignment not found")
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	var student models.Student
	if err := database.DB.Where("course_id = ? AND gitea_id = ?", assignment.CourseID, user.GiteaID).First(&student).Error; err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "not enrolled in this course")
	}

	var existingSubmission models.Submission
	if err := database.DB.Where("assignment_id = ? AND student_id = ?", assignment.ID, student.ID).First(&existingSubmission).Error; err == nil {
		return c.JSON(http.StatusOK, existingSubmission)
	}

	var instructor models.User
	err = database.DB.
		Joins("JOIN course_instructors ON course_instructors.user_id = users.id").
		Where("course_instructors.course_id = ?", assignment.CourseID).
		First(&instructor).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no instructor found for course")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	repoName := fmt.Sprintf("%s-%s", slugify(assignment.Title), user.Username)
	var repoURL string

	if assignment.TemplateRepo != "" {
		parts := strings.Split(assignment.TemplateRepo, "/")
		if len(parts) >= 2 {
			templateOwner := parts[len(parts)-2]
			templateRepo := parts[len(parts)-1]

			repo, err := giteaService.CreateRepoFromTemplate(
				assignment.Course.OrgName,
				templateOwner,
				templateRepo,
				repoName,
				true,
			)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create repository from template: %v", err))
			}
			repoURL = repo.HTMLURL
		}
	} else {
		repo, err := giteaService.CreateOrgRepo(
			assignment.Course.OrgName,
			repoName,
			fmt.Sprintf("Assignment: %s", assignment.Title),
			true,
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create repository")
		}
		repoURL = repo.HTMLURL
	}

	giteaService.AddCollaborator(assignment.Course.OrgName, repoName, user.Username, gitea.AccessModeWrite)

	if assignment.AcademicYear > 0 {
		teamName := fmt.Sprintf("%d-%s-instructors", assignment.AcademicYear, assignment.Course.Slug)
		team, err := giteaService.GetTeamByName(assignment.Course.OrgName, teamName)
		if err == nil && team != nil {
			giteaService.AddTeamRepository(team.ID, assignment.Course.OrgName, repoName)
		}
	}

	// Create webhook for review system if configured
	if h.cfg.WebhookBaseURL != "" && h.cfg.GiteaWebhookSecret != "" {
		webhookURL := fmt.Sprintf("%s/api/webhooks/gitea", h.cfg.WebhookBaseURL)
		go func() {
			giteaService.CreateRepoWebhook(
				assignment.Course.OrgName,
				repoName,
				webhookURL,
				h.cfg.GiteaWebhookSecret,
				[]string{"pull_request_comment", "pull_request_review"},
			)
		}()
	}

	go func() {
		giteaService.SetupFeedbackBranch(assignment.Course.OrgName, repoName)
	}()

	submission := models.Submission{
		AssignmentID: uint(assignmentID),
		StudentID:    student.ID,
		RepoURL:      repoURL,
		Status:       "in_progress",
	}

	if err := database.DB.Create(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create submission")
	}

	return c.JSON(http.StatusCreated, submission)
}

func (h *SubmissionHandler) List(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	assignmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid assignment id")
	}

	var assignment models.Assignment
	if err := database.DB.First(&assignment, assignmentID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "assignment not found")
	}

	if !isInstructor(userID, assignment.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can view all submissions")
	}

	var submissions []models.Submission
	if err := database.DB.Where("assignment_id = ?", assignmentID).Preload("Student").Find(&submissions).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch submissions")
	}

	return c.JSON(http.StatusOK, submissions)
}

func (h *SubmissionHandler) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("submissionId"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid submission id")
	}

	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment").First(&submission, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}

	return c.JSON(http.StatusOK, submission)
}

type GradeRequest struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

func (h *SubmissionHandler) Grade(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	id, err := strconv.ParseUint(c.Param("submissionId"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid submission id")
	}

	var submission models.Submission
	if err := database.DB.Preload("Assignment").First(&submission, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}

	if !isInstructor(userID, submission.Assignment.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can grade submissions")
	}

	var req GradeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.Score < 0 || req.Score > submission.Assignment.MaxPoints {
		return echo.NewHTTPError(http.StatusBadRequest, "score out of range")
	}

	submission.Score = &req.Score
	submission.Feedback = req.Feedback
	submission.Status = "graded"
	now := time.Now()
	submission.SubmittedAt = &now

	if err := database.DB.Save(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to grade submission")
	}

	return c.JSON(http.StatusOK, submission)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
