package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type AssignmentHandler struct {
	cfg *config.Config
}

func NewAssignmentHandler(cfg *config.Config) *AssignmentHandler {
	return &AssignmentHandler{cfg: cfg}
}

type CreateAssignmentRequest struct {
	Title        string `json:"title" validate:"required"`
	Description  string `json:"description"`
	TemplateRepo string `json:"template_repo"`
	Deadline     string `json:"deadline" validate:"required"`
	MaxPoints    int    `json:"max_points"`
	AcademicYear int    `json:"academic_year" validate:"required"`
}

func (h *AssignmentHandler) Create(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if !isInstructor(userID, course.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can create assignments")
	}

	var req CreateAssignmentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.AcademicYear == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "academic_year is required")
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, user.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	_, err = giteaService.GetOrCreateInstructorTeam(course.OrgName, course.Slug, req.AcademicYear, user.Username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create instructor team: "+err.Error())
	}

	assignment := models.Assignment{
		CourseID:     course.ID,
		Title:        req.Title,
		Description:  req.Description,
		TemplateRepo: req.TemplateRepo,
		MaxPoints:    req.MaxPoints,
		AcademicYear: req.AcademicYear,
	}

	if req.Deadline != "" {
		deadline, err := parseDateTime(req.Deadline)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid deadline format")
		}
		assignment.Deadline = deadline
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create assignment")
	}

	return c.JSON(http.StatusCreated, assignment)
}

func (h *AssignmentHandler) List(c echo.Context) error {
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).Preload("Assignments").First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	return c.JSON(http.StatusOK, course.Assignments)
}

func (h *AssignmentHandler) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid assignment id")
	}

	var assignment models.Assignment
	if err := database.DB.Preload("Course").Preload("Submissions").Preload("Submissions.Student").First(&assignment, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "assignment not found")
	}

	return c.JSON(http.StatusOK, assignment)
}

type UpdateAssignmentRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	TemplateRepo string `json:"template_repo"`
	Deadline     string `json:"deadline"`
	MaxPoints    int    `json:"max_points"`
	AcademicYear int    `json:"academic_year"`
}

func (h *AssignmentHandler) Update(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid assignment id")
	}

	var assignment models.Assignment
	if err := database.DB.First(&assignment, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "assignment not found")
	}

	if !isInstructor(userID, assignment.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can update assignments")
	}

	var req UpdateAssignmentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.Title != "" {
		assignment.Title = req.Title
	}
	if req.Description != "" {
		assignment.Description = req.Description
	}
	if req.TemplateRepo != "" {
		assignment.TemplateRepo = req.TemplateRepo
	}
	if req.MaxPoints > 0 {
		assignment.MaxPoints = req.MaxPoints
	}
	if req.AcademicYear > 0 {
		assignment.AcademicYear = req.AcademicYear
	}
	if req.Deadline != "" {
		deadline, err := parseDateTime(req.Deadline)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid deadline format")
		}
		assignment.Deadline = deadline
	}

	if err := database.DB.Save(&assignment).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update assignment")
	}

	return c.JSON(http.StatusOK, assignment)
}

func (h *AssignmentHandler) Delete(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid assignment id")
	}

	var assignment models.Assignment
	if err := database.DB.First(&assignment, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "assignment not found")
	}

	if !isInstructor(userID, assignment.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can delete assignments")
	}

	if err := database.DB.Delete(&models.Assignment{}, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete assignment")
	}

	return c.NoContent(http.StatusNoContent)
}
