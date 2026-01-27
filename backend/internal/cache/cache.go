package cache

import (
	"sync"
	"time"
)

type PendingReview struct {
	ReviewRequestID uint
	SubmissionID    uint
	ExpiresAt       time.Time
}

type ReviewCache struct {
	mu      sync.RWMutex
	pending map[uint]*PendingReview // key: reviewRequestID
}

func NewReviewCache() *ReviewCache {
	return &ReviewCache{
		pending: make(map[uint]*PendingReview),
	}
}

func (c *ReviewCache) Add(reviewRequestID, submissionID uint, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pending[reviewRequestID] = &PendingReview{
		ReviewRequestID: reviewRequestID,
		SubmissionID:    submissionID,
		ExpiresAt:       time.Now().Add(ttl),
	}
}

func (c *ReviewCache) Remove(reviewRequestID uint) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.pending[reviewRequestID]; exists {
		delete(c.pending, reviewRequestID)
		return true
	}
	return false
}

func (c *ReviewCache) Get(reviewRequestID uint) (*PendingReview, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pr, exists := c.pending[reviewRequestID]
	return pr, exists
}

func (c *ReviewCache) Exists(reviewRequestID uint) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.pending[reviewRequestID]
	return exists
}

func (c *ReviewCache) GetExpired() []*PendingReview {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expired []*PendingReview
	now := time.Now()

	for id, pr := range c.pending {
		if now.After(pr.ExpiresAt) {
			expired = append(expired, pr)
			delete(c.pending, id)
		}
	}

	return expired
}

func (c *ReviewCache) GetTimeRemaining(reviewRequestID uint) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if pr, exists := c.pending[reviewRequestID]; exists {
		remaining := time.Until(pr.ExpiresAt)
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return 0
}
