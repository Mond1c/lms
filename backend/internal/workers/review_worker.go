package workers

import (
	"log"
	"time"

	"github.com/Mond1c/gitea-classroom/internal/cache"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"gorm.io/gorm"
)

type ReviewWorker struct {
	cache    *cache.ReviewCache
	sheets   *services.SheetsService
	db       *gorm.DB
	ticker   *time.Ticker
	stopChan chan struct{}
}

func NewReviewWorker(reviewCache *cache.ReviewCache, sheets *services.SheetsService) *ReviewWorker {
	return &ReviewWorker{
		cache:    reviewCache,
		sheets:   sheets,
		db:       database.DB,
		stopChan: make(chan struct{}),
	}
}

func (w *ReviewWorker) Start() {
	w.ticker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.processExpiredReviews()
			case <-w.stopChan:
				w.ticker.Stop()
				return
			}
		}
	}()

	log.Println("Review worker started")
}

func (w *ReviewWorker) Stop() {
	close(w.stopChan)
	log.Println("Review worker stopped")
}

func (w *ReviewWorker) processExpiredReviews() {
	expired := w.cache.GetExpired()

	for _, pr := range expired {
		if err := w.submitToSheets(pr.ReviewRequestID); err != nil {
			log.Printf("Failed to submit review request %d to sheets: %v", pr.ReviewRequestID, err)
			// Optionally re-add to cache for retry
			continue
		}
	}
}

func (w *ReviewWorker) submitToSheets(reviewRequestID uint) error {
	var reviewRequest models.ReviewRequest
	if err := w.db.Preload("Submission.Student").Preload("Submission.Assignment.Course").First(&reviewRequest, reviewRequestID).Error; err != nil {
		return err
	}

	// Skip if not in pending status (might have been cancelled)
	if reviewRequest.Status != models.ReviewStatusPending {
		return nil
	}

	// Skip if sheets service is not configured
	if w.sheets == nil {
		log.Printf("Sheets service not configured, skipping sheet submission for review request %d", reviewRequestID)
		// Still update status to submitted
		now := time.Now()
		reviewRequest.Status = models.ReviewStatusSubmitted
		reviewRequest.SubmittedAt = &now
		return w.db.Save(&reviewRequest).Error
	}

	// Add to Google Sheets
	fullName := reviewRequest.Submission.Student.FullName
	if fullName == "" {
		fullName = reviewRequest.Submission.Student.Username
	}

	rowID, err := w.sheets.AppendReviewRequest(
		fullName,
		reviewRequest.Submission.RepoURL,
		reviewRequest.RequestedAt,
	)
	if err != nil {
		return err
	}

	// Update review request
	now := time.Now()
	reviewRequest.Status = models.ReviewStatusSubmitted
	reviewRequest.SubmittedAt = &now
	reviewRequest.SheetRowID = rowID

	return w.db.Save(&reviewRequest).Error
}
