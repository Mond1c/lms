package handlers

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type InviteHandler struct {
	cfg *config.Config
}

func NewInviteHandler(cfg *config.Config) *InviteHandler {
	return &InviteHandler{cfg: cfg}
}

// Import students from CSV or JSON list
func (h *InviteHandler) ImportStudents(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	// Get course and verify instructor
	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if !isInstructor(userID, course.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can import students")
	}

	// Parse request - can be CSV file or JSON array
	contentType := c.Request().Header.Get("Content-Type")

	var fullNames []string

	if strings.Contains(contentType, "multipart/form-data") {
		// CSV file upload
		file, err := c.FormFile("file")
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "file is required")
		}

		src, err := file.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to open file")
		}
		defer src.Close()

		reader := csv.NewReader(src)
		records, err := reader.ReadAll()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to parse CSV")
		}

		// Skip header if present
		startIdx := 0
		if len(records) > 0 && (records[0][0] == "ФИО" || records[0][0] == "Full Name" || records[0][0] == "Name") {
			startIdx = 1
		}

		for i := startIdx; i < len(records); i++ {
			if len(records[i]) > 0 && strings.TrimSpace(records[i][0]) != "" {
				fullNames = append(fullNames, strings.TrimSpace(records[i][0]))
			}
		}
	} else {
		// JSON array
		var req struct {
			Students []string `json:"students"`
		}
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
		}
		fullNames = req.Students
	}

	if len(fullNames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no students provided")
	}

	// Create invites (no personal tokens, just records)
	var invites []models.StudentInvite
	for _, fullName := range fullNames {
		// Check if already exists
		var existing models.StudentInvite
		err := database.DB.Where("course_id = ? AND full_name = ?", course.ID, fullName).First(&existing).Error
		if err == nil {
			// Already exists, skip
			continue
		}

		invite := models.StudentInvite{
			CourseID: course.ID,
			FullName: fullName,
			Token:    nil, // No personal token needed
			Used:     false,
		}

		if err := database.DB.Create(&invite).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create invite for %s", fullName))
		}

		invites = append(invites, invite)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message": fmt.Sprintf("Added %d students", len(invites)),
		"invites": invites,
	})
}

// List all invites for a course
func (h *InviteHandler) ListInvites(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	slug := c.Param("slug")

	var course models.Course
	if err := database.DB.Where("slug = ?", slug).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "course not found")
	}

	if !isInstructor(userID, course.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can view invites")
	}

	var invites []models.StudentInvite
	if err := database.DB.Where("course_id = ?", course.ID).Preload("Student").Find(&invites).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch invites")
	}

	return c.JSON(http.StatusOK, invites)
}

// Get available students for registration (by course invite code)
func (h *InviteHandler) GetAvailableStudents(c echo.Context) error {
	inviteCode := c.Param("code")

	// Find course by invite code
	var course models.Course
	if err := database.DB.Where("invite_code = ?", inviteCode).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invalid invite code")
	}

	// Get unused invites
	var invites []models.StudentInvite
	if err := database.DB.Where("course_id = ? AND used = ?", course.ID, false).Find(&invites).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch students")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"course_name": course.Name,
		"course_slug": course.Slug,
		"students":    invites,
	})
}

// Register student by selecting from list
func (h *InviteHandler) RegisterStudent(c echo.Context) error {
	inviteCode := c.Param("code")

	var req struct {
		InviteID uint   `json:"invite_id" validate:"required"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	// Find course by invite code
	var course models.Course
	if err := database.DB.Where("invite_code = ?", inviteCode).First(&course).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invalid invite code")
	}

	// Find invite
	var invite models.StudentInvite
	if err := database.DB.Where("id = ? AND course_id = ?", req.InviteID, course.ID).First(&invite).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invalid student selection")
	}

	if invite.Used {
		return echo.NewHTTPError(http.StatusBadRequest, "this student has already registered")
	}

	// Generate username from full name
	username := generateUsername(invite.FullName)

	// Create account in Gitea using admin token
	if h.cfg.GiteaAdminToken == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gitea admin token not configured")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize Gitea service")
	}

	// Create Gitea user
	giteaUser, err := giteaService.CreateUser(username, req.Email, req.Password, invite.FullName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create Gitea account: %v", err))
	}

	// Create Student in LMS database
	student := models.Student{
		CourseID: course.ID,
		GiteaID:  giteaUser.ID,
		Username: giteaUser.UserName,
		Email:    giteaUser.Email,
		FullName: giteaUser.FullName,
	}

	if err := database.DB.Create(&student).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create student record")
	}

	// Mark invite as used
	now := time.Now()
	invite.Used = true
	invite.UsedAt = &now
	invite.StudentID = &student.ID

	if err := database.DB.Save(&invite).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update invite")
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message":  "Account created successfully",
		"username": username,
		"redirect": fmt.Sprintf("%s/api/auth/login", h.cfg.GiteaURL),
	})
}

// Generate username from full name
// Example: "Иванов Иван Иванович" -> "ivanov_ii"
func generateUsername(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "student"
	}

	username := strings.ToLower(transliterate(parts[0]))

	// Add initials
	for i := 1; i < len(parts) && i < 3; i++ {
		if len(parts[i]) > 0 {
			username += "_" + strings.ToLower(string(transliterate(parts[i])[0]))
		}
	}

	// Add random suffix to avoid collisions
	suffix := make([]byte, 3)
	rand.Read(suffix)
	username += hex.EncodeToString(suffix)[:4]

	return username
}

// Simple transliteration for Russian to Latin
func transliterate(s string) string {
	translitMap := map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
		'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
		'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
		'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
		'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
		'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D", 'Е': "E", 'Ё': "E",
		'Ж': "Zh", 'З': "Z", 'И': "I", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M",
		'Н': "N", 'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U",
		'Ф': "F", 'Х': "H", 'Ц': "Ts", 'Ч': "Ch", 'Ш': "Sh", 'Щ': "Sch",
		'Ъ': "", 'Ы': "Y", 'Ь': "", 'Э': "E", 'Ю': "Yu", 'Я': "Ya",
	}

	var result strings.Builder
	for _, r := range s {
		if val, ok := translitMap[r]; ok {
			result.WriteString(val)
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	return result.String()
}
