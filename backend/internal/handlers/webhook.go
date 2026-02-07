package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/cache"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/labstack/echo/v4"
)

type WebhookHandler struct {
	cfg    *config.Config
	cache  *cache.ReviewCache
	sheets *services.SheetsService
}

func NewWebhookHandler(cfg *config.Config, reviewCache *cache.ReviewCache, sheets *services.SheetsService) *WebhookHandler {
	return &WebhookHandler{
		cfg:    cfg,
		cache:  reviewCache,
		sheets: sheets,
	}
}

// Gitea webhook payloads
type GiteaPullRequestPayload struct {
	Action      string `json:"action"` // "opened", "closed", "review_requested", etc.
	Number      int64  `json:"number"`
	PullRequest struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		User  struct {
			ID       int64  `json:"id"`
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"user"`
		Base struct {
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

type GiteaIssueCommentPayload struct {
	Action string `json:"action"` // "created", "edited", "deleted"
	IsPull bool   `json:"is_pull"` // Top-level field indicating if this is a PR comment
	Issue  struct {
		ID     int64  `json:"id"`
		Number int64  `json:"number"`
		Title  string `json:"title"`
		User   struct {
			ID       int64  `json:"id"`
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"user"`
	} `json:"issue"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			ID       int64  `json:"id"`
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"user"`
	} `json:"comment"`
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

	log.Printf("%v", eventType)

	switch eventType {
	case "pull_request":
		return h.handlePullRequest(c, body)
	case "pull_request_review":
		return h.handlePullRequestReview(c, body)
	case "issue_comment":
		return h.handleIssueComment(c, body)
	default:
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}
}

// Handle pull_request events (review_requested)
func (h *WebhookHandler) handlePullRequest(c echo.Context, body []byte) error {
	var payload GiteaPullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	// Only process review_requested action
	if payload.Action != "review_requested" {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "ignored"})
	}

	// Only process Feedback PR
	if payload.PullRequest.Title != "Feedback" {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "ignored"})
	}

	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Check if student is the one requesting review
	requesterUsername := payload.Sender.Username
	if requesterUsername == "" {
		requesterUsername = payload.Sender.Login
	}

	if submission.Student.Username != requesterUsername {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "not_submission_owner"})
	}

	// Check for existing active review request
	var existingRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).First(&existingRequest).Error
	if err == nil {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "review_already_active"})
	}

	// Get instructor to use their token for branch protection
	var instructor models.User
	err = database.DB.
		Joins("JOIN course_instructors ON course_instructors.user_id = users.id").
		Where("course_instructors.course_id = ?", submission.Assignment.CourseID).
		First(&instructor).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no instructor found")
	}

	giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, instructor.AccessToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize gitea service")
	}

	// Enable branch protection
	repoName := extractRepoName(submission.RepoURL)
	orgName := submission.Assignment.Course.OrgName
	if err := giteaService.EnableBranchProtection(orgName, repoName, "main"); err != nil {
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

	fmt.Printf("Review request created: ID=%d, Submission=%d, TTL=%v minutes, Expires at=%v\n",
		reviewRequest.ID, submission.ID, h.cfg.ReviewPendingMinutes, reviewRequest.RequestedAt.Add(ttl))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":            "review_requested",
		"review_request_id": reviewRequest.ID,
		"cancel_deadline":   reviewRequest.RequestedAt.Add(ttl),
	})
}

// Handle pull_request_review events (submitted reviews)
func (h *WebhookHandler) handlePullRequestReview(c echo.Context, body []byte) error {
	var payload GiteaPullRequestReviewPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	// Only process submitted reviews
	if payload.Action != "submitted" {
		return echo.NewHTTPError(http.StatusOK, map[string]string{"status": "ignored"})
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
	err := database.DB.Where("submission_id = ? AND status = ?", submission.ID, models.ReviewStatusSubmitted).
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

	// Restore student's write access using admin token
	repoName := extractRepoName(submission.RepoURL)
	orgName := submission.Assignment.Course.OrgName

	if h.cfg.GiteaAdminToken != "" {
		adminService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
		if err == nil {
			// Restore Write access to student
			if err := adminService.AddCollaborator(orgName, repoName, submission.Student.Username, gitea.AccessModeWrite); err != nil {
				log.Printf("Warning: failed to restore student write access: %v", err)
			} else {
				log.Printf("Restored write access for student %s on %s/%s", submission.Student.Username, orgName, repoName)
			}
		}
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

// Handle issue_comment events (magic command /review)
func (h *WebhookHandler) handleIssueComment(c echo.Context, body []byte) error {
	var payload GiteaIssueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	log.Print(1)
	// Only process created comments
	if payload.Action != "created" {
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}
	log.Print(2)

	// Only process comments on pull requests
	if !payload.IsPull {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_pull_request"})
	}
	log.Print(3)

	// Only process Feedback PR
	if payload.Issue.Title != "Feedback" {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_feedback_pr"})
	}
	log.Print(4)

	// Check if comment contains magic command
	commentBody := strings.TrimSpace(payload.Comment.Body)
	if commentBody != "/review" && commentBody != "@review" {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_review_command"})
	}
	log.Print(5)

	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}
	log.Print(6)

	// Check if commenter is the student who owns this submission
	commenterUsername := payload.Comment.User.Username
	if commenterUsername == "" {
		commenterUsername = payload.Comment.User.Login
	}

	if submission.Student.Username != commenterUsername {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_submission_owner"})
	}
	log.Print(7)

	// Check for existing active review request
	var existingRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).First(&existingRequest).Error
	if err == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "review_already_active"})
	}
	log.Print(8)

	// Use admin token to manage repository access
	if h.cfg.GiteaAdminToken != "" {
		giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
		if err == nil {
			repoName := extractRepoName(submission.RepoURL)
			orgName := submission.Assignment.Course.OrgName

			// Change student's access from Write to Read
			if err := giteaService.AddCollaborator(orgName, repoName, submission.Student.Username, gitea.AccessModeRead); err != nil {
				log.Printf("Warning: failed to change student access to read-only: %v", err)
			} else {
				log.Printf("Changed student %s access to read-only on %s/%s", submission.Student.Username, orgName, repoName)
			}
		}
	}
	log.Print(9)

	// Create review request
	reviewRequest := models.ReviewRequest{
		SubmissionID: submission.ID,
		Status:       models.ReviewStatusPending,
		RequestedAt:  time.Now(),
	}

	if err := database.DB.Create(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create review request")
	}
	log.Print(11)

	// Add to cache with TTL
	ttl := time.Duration(h.cfg.ReviewPendingMinutes) * time.Minute
	h.cache.Add(reviewRequest.ID, submission.ID, ttl)

	fmt.Printf("Review request created: ID=%d, Submission=%d, TTL=%v minutes, Expires at=%v\n",
		reviewRequest.ID, submission.ID, h.cfg.ReviewPendingMinutes, reviewRequest.RequestedAt.Add(ttl))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":            "review_requested",
		"review_request_id": reviewRequest.ID,
		"cancel_deadline":   reviewRequest.RequestedAt.Add(ttl),
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
