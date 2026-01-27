package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type WebhookHandler struct {
	cfg    *config.Config
	sheets *services.SheetsService
}

func NewWebhookHandler(cfg *config.Config, sheets *services.SheetsService) *WebhookHandler {
	return &WebhookHandler{
		cfg:    cfg,
		sheets: sheets,
	}
}

// GiteaWebhookPayload represents the webhook payload from Gitea
type GiteaPullRequestReviewPayload struct {
	Action string `json:"action"` // "submitted", "edited", "dismissed"
	Review struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			ID       int64  `json:"id"`
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"user"`
		State string `json:"state"` // "APPROVED", "COMMENT", "REQUEST_CHANGES"
	} `json:"review"`
	PullRequest struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Base  struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Repository struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender struct {
		ID       int64  `json:"id"`
		Login    string `json:"login"`
		Username string `json:"username"`
	} `json:"sender"`
}

func (h *WebhookHandler) HandleGiteaWebhook(c echo.Context) error {
	// Read body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to read body")
	}

	// Verify signature if secret is configured
	if h.cfg.GiteaWebhookSecret != "" {
		signature := c.Request().Header.Get("X-Gitea-Signature")
		if !verifySignature(body, signature, h.cfg.GiteaWebhookSecret) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid signature")
		}
	}

	// Check event type
	eventType := c.Request().Header.Get("X-Gitea-Event")
	if eventType != "pull_request_review" {
		// Ignore other events
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}

	var payload GiteaPullRequestReviewPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	// Only process submitted reviews
	if payload.Action != "submitted" {
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}

	// Only process reviews on Feedback PR
	if payload.PullRequest.Title != "Feedback" {
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}

	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		// Submission not found, might be a repo we don't track
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Find active review request for this submission
	var reviewRequest models.ReviewRequest
	err = database.DB.Where("submission_id = ? AND status = ?", submission.ID, models.ReviewStatusSubmitted).
		First(&reviewRequest).Error
	if err != nil {
		// No active review request
		return c.JSON(http.StatusOK, map[string]string{"status": "no_active_review"})
	}

	// Check if reviewer is an instructor for this course's year
	reviewerUsername := payload.Sender.Username
	if reviewerUsername == "" {
		reviewerUsername = payload.Sender.Login
	}

	// Get instructor token to check team membership
	var instructor models.User
	err = database.DB.
		Joins("JOIN course_instructors ON course_instructors.user_id = users.id").
		Where("course_instructors.course_id = ?", submission.Assignment.CourseID).
		First(&instructor).Error
	if err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "no_instructor"})
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	// Check if reviewer is in instructor team
	teamName := fmt.Sprintf("%d-%s-instructors", submission.Assignment.AcademicYear, submission.Assignment.Course.Slug)
	isMember, err := giteaService.IsTeamMember(submission.Assignment.Course.OrgName, teamName, reviewerUsername)
	if err != nil || !isMember {
		// Reviewer is not an instructor, ignore
		return c.JSON(http.StatusOK, map[string]string{"status": "reviewer_not_instructor"})
	}

	// Disable branch protection
	repoName := extractRepoName(submission.RepoURL)
	orgName := submission.Assignment.Course.OrgName
	if err := giteaService.DisableBranchProtection(orgName, repoName, "main"); err != nil {
		fmt.Printf("Warning: failed to disable branch protection: %v\n", err)
	}

	// Update review request status
	now := time.Now()
	reviewRequest.Status = models.ReviewStatusReviewed
	reviewRequest.ReviewedAt = &now

	if err := database.DB.Save(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update review request")
	}

	// Update Google Sheets status
	if h.sheets != nil && reviewRequest.SheetRowID > 0 {
		h.sheets.UpdateRowStatus(reviewRequest.SheetRowID, "Проверено")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":            "processed",
		"review_request_id": fmt.Sprintf("%d", reviewRequest.ID),
	})
}

func verifySignature(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Signature might have "sha256=" prefix
	signature = strings.TrimPrefix(signature, "sha256=")

	return hmac.Equal([]byte(expectedMAC), []byte(signature))
}
