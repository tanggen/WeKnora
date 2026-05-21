package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitResult indicates the outcome of a rate limit check.
type RateLimitResult int

const (
	// RateLimitOK means the request is within limits.
	RateLimitOK RateLimitResult = iota
	// RateLimitExceeded means the rate limit has been exceeded.
	RateLimitExceeded
	// RateLimitUnavailable means the rate limiter is not configured (no Redis).
	RateLimitUnavailable
)

// RateLimiter implements a simple sliding-window-style rate limiter backed by Redis.
// It uses an INCR + EXPIRE pattern: the key is incremented on each attempt, and
// a TTL is set on first creation. When the key expires the counter resets naturally.
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a RateLimiter backed by a Redis client.
// If client is nil all checks return RateLimitUnavailable (allow-through mode).
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// CheckRateLimit checks whether the action identified by keyPrefix + identifier
// has exceeded maxRequests within a window of windowSeconds. It returns
// RateLimitOK, RateLimitExceeded, or RateLimitUnavailable.
func (r *RateLimiter) CheckRateLimit(
	ctx context.Context,
	keyPrefix string,
	identifier string,
	maxRequests int,
	windowSeconds int,
) (RateLimitResult, error) {
	if r.client == nil {
		return RateLimitUnavailable, nil
	}

	key := fmt.Sprintf("%s:%s", keyPrefix, identifier)
	val, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return RateLimitUnavailable, fmt.Errorf("rate limit incr failed: %w", err)
	}

	// Set expiry on first request (val == 1)
	if val == 1 {
		if err := r.client.Expire(ctx, key, time.Duration(windowSeconds)*time.Second).Err(); err != nil {
			return RateLimitUnavailable, fmt.Errorf("rate limit expire set failed: %w", err)
		}
	}

	if val > int64(maxRequests) {
		return RateLimitExceeded, nil
	}
	return RateLimitOK, nil
}

// ResetRateLimit removes the rate limit counter for the given key/identifier.
// Used to clear login failure counters after a successful login.
func (r *RateLimiter) ResetRateLimit(ctx context.Context, keyPrefix, identifier string) error {
	if r.client == nil {
		return nil
	}
	key := fmt.Sprintf("%s:%s", keyPrefix, identifier)
	return r.client.Del(ctx, key).Err()
}
