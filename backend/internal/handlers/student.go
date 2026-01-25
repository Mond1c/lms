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

type StudentHandler struct {
	cfg *config.Config
}

func NewStudentHandler(cfg *config.Config) *StudentHandler {
	return &StudentHandler{cfg: cfg}
}

func (h *StudentHandler) List(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).Preload("Students").First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if !isInstructor(userID, course.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can view student list")
	}

	return c.JSON(http.StatusOK, course.Students)
}

func (h *StudentHandler) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid student id")
	}

	var student models.Student
	if err := database.DB.Preload("Submissions").Preload("Submissions.Assignment").First(&student, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "student not found")
	}

	return c.JSON(http.StatusOK, student)
}

func (h *StudentHandler) Enroll(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	var existingStudent models.Student
	if err := database.DB.Where("course_id = ? AND gitea_id = ?", course.ID, user.GiteaID).First(&existingStudent).Error; err == nil {
		return echo.NewHTTPError(http.StatusConflict, "already enrolled")
	}

	student := models.Student{
		CourseID: course.ID,
		GiteaID:  user.GiteaID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
	}

	if err := database.DB.Create(&student).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to enroll")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, user.AccessToken)
	if err == nil {
		teams, err := giteaService.GetOrgTeams(course.OrgName)
		if err == nil {
			for _, team := range teams {
				if team.Name == "Students" {
					giteaService.AddTeamMember(team.ID, user.Username)
					break
				}
			}
		}
	}

	return c.JSON(http.StatusCreated, student)
}

func (h *StudentHandler) Remove(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid student id")
	}

	var student models.Student
	if err := database.DB.First(&student, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "student not found")
	}

	if !isInstructor(userID, student.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can remove students")
	}

	if err := database.DB.Delete(&models.Student{}, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove student")
	}

	return c.NoContent(http.StatusNoContent)
}
