package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/labstack/echo/v4"
)

type CourseHandler struct {
	cfg *config.Config
}

func NewCourseHandler(cfg *config.Config) *CourseHandler {
	return &CourseHandler{cfg: cfg}
}

type CreateCourseRequest struct {
	Name         string `json:"name" validate:"required"`
	Description  string `json:"description"`
	OrgName      string `json:"org_name" validate:"required"`
	AcademicYear int    `json:"academic_year" validate:"required"`
}

func (h *CourseHandler) Create(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	if !user.IsAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "only admins can create courses")
	}

	var req CreateCourseRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	// Create slug with academic year to ensure uniqueness
	baseName := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	slug := fmt.Sprintf("%s-%d", baseName, req.AcademicYear)

	inviteCode := generateInviteCode()

	course := models.Course{
		Name:         req.Name,
		Description:  req.Description,
		Slug:         slug,
		OrgName:      req.OrgName,
		AcademicYear: req.AcademicYear,
		InviteCode:   inviteCode,
		Instructors:  []models.User{user},
	}

	if err := database.DB.Create(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create course")
	}

	return c.JSON(http.StatusCreated, course)
}

func (h *CourseHandler) List(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var instructorCourses []models.Course
	database.DB.
		Joins("JOIN course_instructors ON course_instructors.course_id = courses.id").
		Where("course_instructors.user_id = ?", userID).
		Preload("Instructors").
		Find(&instructorCourses)

	return c.JSON(http.StatusOK, instructorCourses)
}

func (h *CourseHandler) ListEnrolled(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	var studentRecords []models.Student
	database.DB.Where("gitea_id = ?", user.GiteaID).Find(&studentRecords)

	var courseIDs []uint
	for _, s := range studentRecords {
		courseIDs = append(courseIDs, s.CourseID)
	}

	var enrolledCourses []models.Course
	if len(courseIDs) > 0 {
		database.DB.Where("id IN ?", courseIDs).Find(&enrolledCourses)
	}

	return c.JSON(http.StatusOK, enrolledCourses)
}

type CourseResponse struct {
	models.Course
	IsInstructor bool `json:"is_instructor"`
}

func (h *CourseHandler) Get(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	err := database.DB.
		Where("slug = ?", slug).
		Preload("Instructors").
		Preload("Assignments").
		Preload("Students").
		First(&course).Error

	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if course.InviteCode == "" {
		course.InviteCode = generateInviteCode()
		database.DB.Save(&course)
	}

	response := CourseResponse{
		Course:       course,
		IsInstructor: isInstructor(userID, course.ID),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *CourseHandler) GetByInviteCode(c echo.Context) error {
	code := c.Param("code")

	var course models.Course
	err := database.DB.
		Where("invite_code = ?", code).
		First(&course).Error

	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invalid invite code")
	}

	return c.JSON(http.StatusOK, course)
}

func (h *CourseHandler) RegenerateInviteCode(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if !isInstructor(userID, course.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can regenerate invite code")
	}

	course.InviteCode = generateInviteCode()
	if err := database.DB.Save(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to regenerate invite code")
	}

	return c.JSON(http.StatusOK, map[string]string{"invite_code": course.InviteCode})
}

func generateInviteCode() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
