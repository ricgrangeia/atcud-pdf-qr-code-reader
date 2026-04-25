package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// llmPaths lists the endpoints that hit the upstream tool server / LLM.
// Requests to these paths are also charged against the stricter LLM bucket.
var llmPaths = []string{
	"/api/v1/document/parse/enriched",
	"/api/v1/image/parse/enriched",
	"/api/v1/document/full",
	"/api/v1/document/items",
	"/api/v1/nif/lookup/bulk",
}

func isLLMPath(p string) bool {
	for _, prefix := range llmPaths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// ipLimiter is a simple per-IP token-bucket rate limiter.
// Each IP gets its own *rate.Limiter; idle entries are evicted periodically.
type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipEntry
	r        rate.Limit
	burst    int
	ttl      time.Duration
}

type ipEntry struct {
	limiter *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(perMinute int, burst int) *ipLimiter {
	l := &ipLimiter{
		limiters: make(map[string]*ipEntry),
		r:        rate.Every(time.Minute / time.Duration(perMinute)),
		burst:    burst,
		ttl:      30 * time.Minute,
	}
	go l.gc()
	return l
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.limiters[ip]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}
	lim := rate.NewLimiter(l.r, l.burst)
	l.limiters[ip] = &ipEntry{limiter: lim, lastSeen: time.Now()}
	return lim
}

// gc evicts entries that have been idle for longer than ttl.
func (l *ipLimiter) gc() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-l.ttl)
		l.mu.Lock()
		for ip, e := range l.limiters {
			if e.lastSeen.Before(cutoff) {
				delete(l.limiters, ip)
			}
		}
		l.mu.Unlock()
	}
}

// rateLimitMiddleware enforces a per-IP rate limit on every request, plus a stricter
// per-IP limit on requests that hit upstream LLM/tool-server endpoints.
func rateLimitMiddleware(global, llm *ipLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !global.get(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"title":  "Too Many Requests",
				"status": http.StatusTooManyRequests,
				"detail": "rate limit exceeded — please slow down",
			})
			return
		}
		if isLLMPath(c.Request.URL.Path) && !llm.get(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"title":  "Too Many Requests",
				"status": http.StatusTooManyRequests,
				"detail": "AI endpoint rate limit exceeded — please slow down",
			})
			return
		}
		c.Next()
	}
}
