package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/cache"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type ReviewHandler struct {
	cfg    *config.Config
	cache  *cache.ReviewCache
	sheets *services.SheetsService
}

func NewReviewHandler(cfg *config.Config, reviewCache *cache.ReviewCache, sheets *services.SheetsService) *ReviewHandler {
	return &ReviewHandler{
		cfg:    cfg,
		cache:  reviewCache,
		sheets: sheets,
	}
}

func (h *ReviewHandler) RequestReview(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	submissionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid submission id")
	}

	// Get submission with related data
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").First(&submission, submissionID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}

	// Get user to verify ownership
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not found")
	}

	// Verify that the user owns this submission
	if submission.Student.GiteaID != user.GiteaID {
		return echo.NewHTTPError(http.StatusForbidden, "you don't own this submission")
	}

	// Check for existing active review request
	var existingRequest models.ReviewRequest
	err = database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).First(&existingRequest).Error
	if err == nil {
		return echo.NewHTTPError(http.StatusConflict, "active review request already exists")
	}

	// Get instructor to use their token for Gitea operations
	var instructor models.User
	err = database.DB.
		Joins("JOIN course_instructors ON course_instructors.user_id = users.id").
		Where("course_instructors.course_id = ?", submission.Assignment.CourseID).
		First(&instructor).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no instructor found for course")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	// Extract repo name from URL
	repoName := extractRepoName(submission.RepoURL)
	orgName := submission.Assignment.Course.OrgName

	// Enable branch protection to block pushes
	if err := giteaService.EnableBranchProtection(orgName, repoName, "main"); err != nil {
		// Log but don't fail - protection might already exist
		fmt.Printf("Warning: failed to enable branch protection: %v\n", err)
	}

	// Create review request
	reviewRequest := models.ReviewRequest{
		SubmissionID: submission.ID,
		Status:       models.ReviewStatusPending,
		RequestedAt:  time.Now(),
	}

	if err := database.DB.Create(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create review request")
	}

	// Add to cache with TTL
	ttl := time.Duration(h.cfg.ReviewPendingMinutes) * time.Minute
	h.cache.Add(reviewRequest.ID, submission.ID, ttl)

	// Create webhook for this repo if webhook URL is configured
	if h.cfg.WebhookBaseURL != "" && h.cfg.GiteaWebhookSecret != "" {
		webhookURL := fmt.Sprintf("%s/webhooks/gitea", h.cfg.WebhookBaseURL)
		giteaService.CreateRepoWebhook(orgName, repoName, webhookURL, h.cfg.GiteaWebhookSecret, []string{"pull_request_review"})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"review_request":   reviewRequest,
		"cancel_deadline":  reviewRequest.RequestedAt.Add(ttl),
		"seconds_to_cancel": int(ttl.Seconds()),
	})
}

func (h *ReviewHandler) CancelReview(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	reviewRequestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid review request id")
	}

	var reviewRequest models.ReviewRequest
	if err := database.DB.Preload("Submission.Student").Preload("Submission.Assignment.Course").First(&reviewRequest, reviewRequestID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "review request not found")
	}

	// Verify ownership
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not found")
	}

	if reviewRequest.Submission.Student.GiteaID != user.GiteaID {
		return echo.NewHTTPError(http.StatusForbidden, "you don't own this review request")
	}

	// Check if still in pending status
	if reviewRequest.Status != models.ReviewStatusPending {
		return echo.NewHTTPError(http.StatusBadRequest, "can only cancel pending review requests")
	}

	// Check if still in cache (within TTL)
	if !h.cache.Exists(reviewRequest.ID) {
		return echo.NewHTTPError(http.StatusBadRequest, "cancellation period has expired")
	}

	// Get instructor token for Gitea operations
	var instructor models.User
	err = database.DB.
		Joins("JOIN course_instructors ON course_instructors.user_id = users.id").
		Where("course_instructors.course_id = ?", reviewRequest.Submission.Assignment.CourseID).
		First(&instructor).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no instructor found")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	// Disable branch protection
	repoName := extractRepoName(reviewRequest.Submission.RepoURL)
	orgName := reviewRequest.Submission.Assignment.Course.OrgName
	giteaService.DisableBranchProtection(orgName, repoName, "main")

	// Remove from cache
	h.cache.Remove(reviewRequest.ID)

	// Update status
	reviewRequest.Status = models.ReviewStatusCancelled
	if err := database.DB.Save(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update review request")
	}

	return c.JSON(http.StatusOK, reviewRequest)
}

func (h *ReviewHandler) GetReviewStatus(c echo.Context) error {
	submissionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid submission id")
	}

	var reviewRequest models.ReviewRequest
	err = database.DB.Where("submission_id = ? AND status IN ?", submissionID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).
		Order("created_at DESC").First(&reviewRequest).Error

	if err != nil {
		// No active review request
		return c.JSON(http.StatusOK, map[string]interface{}{
			"has_active_request": false,
		})
	}

	response := map[string]interface{}{
		"has_active_request": true,
		"review_request":     reviewRequest,
	}

	// If pending, add time remaining
	if reviewRequest.Status == models.ReviewStatusPending {
		remaining := h.cache.GetTimeRemaining(reviewRequest.ID)
		response["seconds_remaining"] = int(remaining.Seconds())
		response["can_cancel"] = remaining > 0
	}

	return c.JSON(http.StatusOK, response)
}

// MarkReviewed is called by webhook or manually by instructor
func (h *ReviewHandler) MarkReviewed(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	reviewRequestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid review request id")
	}

	var reviewRequest models.ReviewRequest
	if err := database.DB.Preload("Submission.Assignment.Course").First(&reviewRequest, reviewRequestID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "review request not found")
	}

	// Verify instructor
	if !isInstructor(userID, reviewRequest.Submission.Assignment.CourseID) {
		return echo.NewHTTPError(http.StatusForbidden, "only instructors can mark as reviewed")
	}

	if reviewRequest.Status != models.ReviewStatusSubmitted {
		return echo.NewHTTPError(http.StatusBadRequest, "review request is not in submitted status")
	}

	// Get instructor token
	var instructor models.User
	if err := database.DB.First(&instructor, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "instructor not found")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	// Disable branch protection
	repoName := extractRepoName(reviewRequest.Submission.RepoURL)
	orgName := reviewRequest.Submission.Assignment.Course.OrgName
	giteaService.DisableBranchProtection(orgName, repoName, "main")

	// Update status
	now := time.Now()
	reviewRequest.Status = models.ReviewStatusReviewed
	reviewRequest.ReviewedAt = &now

	if err := database.DB.Save(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update review request")
	}

	// Update Google Sheets status if configured
	if h.sheets != nil && reviewRequest.SheetRowID > 0 {
		h.sheets.UpdateRowStatus(reviewRequest.SheetRowID, "Проверено")
	}

	return c.JSON(http.StatusOK, reviewRequest)
}

func extractRepoName(repoURL string) string {
	// Extract repo name from URL like "https://gitea.example.com/org/repo-name"
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
