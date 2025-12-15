package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter tracks request counts per IP address
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int           // Max requests allowed
	window   time.Duration // Time window for rate limiting
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Start cleanup goroutine to prevent memory leak
	go rl.cleanupLoop()

	return rl
}

// Allow checks if request from IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get requests for this IP
	requests := rl.requests[ip]

	// Remove old requests outside time window
	validRequests := []time.Time{}
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if limit exceeded
	if len(validRequests) >= rl.limit {
		rl.requests[ip] = validRequests
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[ip] = validRequests

	return true
}

// cleanupLoop periodically removes old entries to prevent memory leak
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes IPs with no recent requests
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window * 2) // Keep data for 2x window

	for ip, requests := range rl.requests {
		// Check if all requests are old
		allOld := true
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				allOld = false
				break
			}
		}

		// Remove IP if all requests are old
		if allOld {
			delete(rl.requests, ip)
		}
	}
}

// RateLimitAuth creates middleware for auth endpoints
// Limits: 5 requests per 15 minutes per IP
func RateLimitAuth() func(http.HandlerFunc) http.HandlerFunc {
	limiter := NewRateLimiter(5, 15*time.Minute)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get real IP (handle proxies)
			ip := getClientIP(r)

			// Check rate limit
			if !limiter.Allow(ip) {
				slog.Warn("rate limit exceeded",
					"ip", ip,
					"path", r.URL.Path,
				)
				http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next(w, r)
		}
	}
}

// getClientIP extracts real client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take first IP in list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}
