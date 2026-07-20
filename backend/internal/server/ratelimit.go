package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/auth"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

// fixedWindowLimiter is a deliberately small, process-local protection layer.
// It limits abuse before requests reach password hashing or the paid AI
// gateway. A shared store can replace it later if the API runs many replicas.
type fixedWindowLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	limit   int
	window  time.Duration
	now     func() time.Time
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		buckets: make(map[string]rateBucket),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

func (l *fixedWindowLimiter) middleware(key func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := l.now()
		bucketKey := key(c)

		l.mu.Lock()
		if len(l.buckets) > 10_000 {
			for existingKey, existing := range l.buckets {
				if !now.Before(existing.resetAt) {
					delete(l.buckets, existingKey)
				}
			}
		}
		bucket := l.buckets[bucketKey]
		if bucket.resetAt.IsZero() || !now.Before(bucket.resetAt) {
			bucket = rateBucket{resetAt: now.Add(l.window)}
		}
		if bucket.count >= l.limit {
			retryAfter := int(bucket.resetAt.Sub(now).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			l.mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		bucket.count++
		l.buckets[bucketKey] = bucket
		l.mu.Unlock()

		c.Next()
	}
}

func clientIPKey(prefix string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		// RemoteIP deliberately ignores forwarding headers. Trusted proxy
		// configuration can be added with the deployment hardening work;
		// accepting arbitrary X-Forwarded-For would let clients evade limits.
		return prefix + ":ip:" + c.RemoteIP()
	}
}

func userOrIPKey(prefix string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		if userID := auth.GetUserID(c); userID != 0 {
			return fmt.Sprintf("%s:user:%d", prefix, userID)
		}
		return prefix + ":ip:" + c.RemoteIP()
	}
}
