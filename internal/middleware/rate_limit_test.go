package middleware

import (
	"testing"
	"time"
)

func TestRateLimiterRejectsRequestsAboveLimit(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	if !limiter.Allow("127.0.0.1") || !limiter.Allow("127.0.0.1") {
		t.Fatal("requests within the limit should be allowed")
	}
	if limiter.Allow("127.0.0.1") {
		t.Fatal("request above the limit should be rejected")
	}
	if !limiter.Allow("127.0.0.2") {
		t.Fatal("limits must be isolated by IP")
	}
}

func TestRateLimiterSupportsPerPlanLimits(t *testing.T) {
	limiter := NewRateLimiter(600, time.Minute)
	if !limiter.AllowLimit("builder-key", 1) || limiter.AllowLimit("builder-key", 1) {
		t.Fatal("builder key should be limited independently")
	}
	if !limiter.AllowLimit("pro-key", 2) || !limiter.AllowLimit("pro-key", 2) {
		t.Fatal("pro key should receive its own higher allowance")
	}
}
