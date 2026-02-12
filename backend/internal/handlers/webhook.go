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
	Action string `json:"action"`  // "created", "edited", "deleted"
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
	case "pull_request_rejected":
		// Restore access when instructor requests changes
		return h.handleReviewed(c, body)
	case "pull_request_approved":
		// Restore access when instructor approves
		return h.handleReviewed(c, body)
	case "issue_comment":
		return h.handleIssueComment(c, body)
	default:
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}
}

// Handle pull_request events (review_requested, reviewed)
func (h *WebhookHandler) handlePullRequest(c echo.Context, body []byte) error {
	var payload GiteaPullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	// Handle different actions
	if payload.Action == "review_requested" {
		return h.handleReviewRequested(c, payload)
	} else if payload.Action == "reviewed" {
		return h.handleReviewed(c, body)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
}

// Handle review_requested action
func (h *WebhookHandler) handleReviewRequested(c echo.Context, payload GiteaPullRequestPayload) error {

	// Only process Feedback PR
	if payload.PullRequest.Title != "Feedback" {
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
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

// Handle "reviewed" action from pull_request event (when using Request changes/Approve buttons)
func (h *WebhookHandler) handleReviewed(c echo.Context, body []byte) error {
	// Parse as generic map to get review type
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	// Extract repository URL
	repository, ok := payload["repository"].(map[string]interface{})
	if !ok {
		return c.JSON(http.StatusOK, map[string]string{"status": "invalid_repository"})
	}

	repoURL, ok := repository["html_url"].(string)
	if !ok {
		return c.JSON(http.StatusOK, map[string]string{"status": "invalid_repo_url"})
	}

	// Find submission by repo URL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Find active review request for this submission
	var reviewRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).
		Order("created_at DESC").First(&reviewRequest).Error
	if err != nil {
		// No active review request
		return c.JSON(http.StatusOK, map[string]string{"status": "no_active_review"})
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

	log.Printf("Review completed via 'reviewed' action: ReviewRequest=%d, Submission=%d", reviewRequest.ID, submission.ID)

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

	// Handle different commands
	switch commentBody {
	case "/review", "@review":
		return h.handleReviewCommand(c, payload)
	case "/unreview":
		return h.handleUnreviewCommand(c, payload)
	case "/review_now":
		return h.handleReviewNowCommand(c, payload)
	case "/force_unreview":
		return h.handleForceUnreviewCommand(c, payload)
	default:
		return c.JSON(http.StatusOK, map[string]string{"status": "not_review_command"})
	}
}

// Handle /review or @review command
func (h *WebhookHandler) handleReviewCommand(c echo.Context, payload GiteaIssueCommentPayload) error {
	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Check if commenter is the student who owns this submission
	commenterUsername := payload.Comment.User.Username
	if commenterUsername == "" {
		commenterUsername = payload.Comment.User.Login
	}

	if submission.Student.Username != commenterUsername {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_submission_owner"})
	}

	// Check for existing active review request
	var existingRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).First(&existingRequest).Error
	if err == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "review_already_active"})
	}

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

	log.Printf("Review request created: ID=%d, Submission=%d, TTL=%v minutes, Expires at=%v\n",
		reviewRequest.ID, submission.ID, h.cfg.ReviewPendingMinutes, reviewRequest.RequestedAt.Add(ttl))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":            "review_requested",
		"review_request_id": reviewRequest.ID,
		"cancel_deadline":   reviewRequest.RequestedAt.Add(ttl),
	})
}

// Handle /unreview command - cancel review while it's in cache
func (h *WebhookHandler) handleUnreviewCommand(c echo.Context, payload GiteaIssueCommentPayload) error {
	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Check if commenter is the student who owns this submission
	commenterUsername := payload.Comment.User.Username
	if commenterUsername == "" {
		commenterUsername = payload.Comment.User.Login
	}

	if submission.Student.Username != commenterUsername {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_submission_owner"})
	}

	// Find pending review request
	var reviewRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status = ?", submission.ID, models.ReviewStatusPending).
		First(&reviewRequest).Error
	if err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "no_pending_review"})
	}

	// Check if still in cache (within cancel period)
	if !h.cache.Exists(reviewRequest.ID) {
		return c.JSON(http.StatusOK, map[string]string{"status": "review_already_submitted", "message": "Review has been submitted to Google Sheets and cannot be cancelled"})
	}

	// Remove from cache
	h.cache.Remove(reviewRequest.ID)

	// Update review request status
	reviewRequest.Status = models.ReviewStatusCancelled
	if err := database.DB.Save(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to cancel review request")
	}

	// Restore write access
	if h.cfg.GiteaAdminToken != "" {
		giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
		if err == nil {
			repoName := extractRepoName(submission.RepoURL)
			orgName := submission.Assignment.Course.OrgName

			if err := giteaService.AddCollaborator(orgName, repoName, submission.Student.Username, gitea.AccessModeWrite); err != nil {
				log.Printf("Warning: failed to restore student write access: %v", err)
			} else {
				log.Printf("Restored write access for student %s on %s/%s", submission.Student.Username, orgName, repoName)
			}
		}
	}

	log.Printf("Review request cancelled: ID=%d, Submission=%d", reviewRequest.ID, submission.ID)

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "review_cancelled",
		"message": "Review request cancelled, write access restored",
	})
}

// Handle /review_now command - immediately submit to Google Sheets
func (h *WebhookHandler) handleReviewNowCommand(c echo.Context, payload GiteaIssueCommentPayload) error {
	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Check if commenter is the student who owns this submission
	commenterUsername := payload.Comment.User.Username
	if commenterUsername == "" {
		commenterUsername = payload.Comment.User.Login
	}

	if submission.Student.Username != commenterUsername {
		return c.JSON(http.StatusOK, map[string]string{"status": "not_submission_owner"})
	}

	// Check for existing active review request
	var existingRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted}).First(&existingRequest).Error
	if err == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "review_already_active"})
	}

	// Use admin token to block repository immediately
	if h.cfg.GiteaAdminToken != "" {
		giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
		if err == nil {
			repoName := extractRepoName(submission.RepoURL)
			orgName := submission.Assignment.Course.OrgName

			// Change student's access from Write to Read immediately
			if err := giteaService.AddCollaborator(orgName, repoName, submission.Student.Username, gitea.AccessModeRead); err != nil {
				log.Printf("Warning: failed to change student access to read-only: %v", err)
			} else {
				log.Printf("Changed student %s access to read-only on %s/%s (immediate)", submission.Student.Username, orgName, repoName)
			}
		}
	}

	// Create review request and immediately submit to sheets
	now := time.Now()
	reviewRequest := models.ReviewRequest{
		SubmissionID: submission.ID,
		Status:       models.ReviewStatusSubmitted,
		RequestedAt:  now,
		SubmittedAt:  &now,
	}

	if err := database.DB.Create(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create review request")
	}

	// Submit to Google Sheets immediately if available
	if h.sheets != nil {
		studentInfo := fmt.Sprintf("%s (%s)", submission.Student.FullName, submission.Student.Username)
		repoURL := submission.RepoURL
		rowID, err := h.sheets.AppendReviewRequest(studentInfo, repoURL, now)
		if err != nil {
			log.Printf("Warning: failed to add review to Google Sheets: %v", err)
		} else {
			reviewRequest.SheetRowID = rowID
			database.DB.Save(&reviewRequest)
			log.Printf("Review request immediately submitted to Google Sheets: ID=%d, Submission=%d, RowID=%d",
				reviewRequest.ID, submission.ID, rowID)
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":            "review_submitted_immediately",
		"review_request_id": reviewRequest.ID,
		"message":           "Review request submitted immediately to Google Sheets",
	})
}

// Handle /force_unreview command - admin command to cancel any review request
func (h *WebhookHandler) handleForceUnreviewCommand(c echo.Context, payload GiteaIssueCommentPayload) error {
	// Find submission by repo URL
	repoURL := payload.Repository.HTMLURL
	var submission models.Submission
	if err := database.DB.Preload("Student").Preload("Assignment.Course").Where("repo_url = ?", repoURL).First(&submission).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "submission_not_found"})
	}

	// Check if commenter is an instructor for this course
	commenterUsername := payload.Comment.User.Username
	if commenterUsername == "" {
		commenterUsername = payload.Comment.User.Login
	}

	// Get commenter user
	var commenter models.User
	if err := database.DB.Where("username = ?", commenterUsername).First(&commenter).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "commenter_not_found"})
	}

	// Check if commenter is instructor for this course
	if !isInstructor(commenter.ID, submission.Assignment.CourseID) {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "forbidden",
			"message": "Only instructors can use /force_unreview",
		})
	}

	// Find any active review request (pending, submitted, or even reviewed)
	var reviewRequest models.ReviewRequest
	err := database.DB.Where("submission_id = ? AND status IN ?", submission.ID,
		[]string{models.ReviewStatusPending, models.ReviewStatusSubmitted, models.ReviewStatusReviewed}).
		Order("created_at DESC").First(&reviewRequest).Error
	if err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "no_review_request"})
	}

	// Remove from cache if exists
	h.cache.Remove(reviewRequest.ID)

	// Update review request status
	reviewRequest.Status = models.ReviewStatusCancelled
	if err := database.DB.Save(&reviewRequest).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to cancel review request")
	}

	// Restore write access using admin token
	if h.cfg.GiteaAdminToken != "" {
		giteaService, err := services.NewGiteaService(h.cfg.GiteaURL, h.cfg.GiteaAdminToken)
		if err == nil {
			repoName := extractRepoName(submission.RepoURL)
			orgName := submission.Assignment.Course.OrgName

			if err := giteaService.AddCollaborator(orgName, repoName, submission.Student.Username, gitea.AccessModeWrite); err != nil {
				log.Printf("Warning: failed to restore student write access: %v", err)
			} else {
				log.Printf("Restored write access for student %s on %s/%s (forced by instructor %s)",
					submission.Student.Username, orgName, repoName, commenterUsername)
			}
		}
	}

	// Remove from Google Sheets if it was submitted
	if h.sheets != nil && reviewRequest.SheetRowID > 0 {
		if err := h.sheets.DeleteRow(reviewRequest.SheetRowID); err != nil {
			log.Printf("Warning: failed to remove from Google Sheets: %v", err)
		}
	}

	log.Printf("Review request force-cancelled by instructor %s: ID=%d, Submission=%d, Status was=%s",
		commenterUsername, reviewRequest.ID, submission.ID, reviewRequest.Status)

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "force_cancelled",
		"message": fmt.Sprintf("Review request cancelled by instructor %s, write access restored", commenterUsername),
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
