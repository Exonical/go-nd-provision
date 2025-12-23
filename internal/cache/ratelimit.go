package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

var _ valkey.Client // ensure import is used

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Current   int64
	Limit     int64
	Remaining int64
	ResetAt   time.Time
}

// CheckRateLimit checks if a request is allowed under the rate limit
// Returns the result and whether the request should proceed
func (v *ValkeyClient) CheckRateLimit(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error) {
	// IncrWithTTL returns count and remaining TTL atomically (single RTT)
	current, remainingTTL, err := v.IncrWithTTL(ctx, key, window)
	if err != nil {
		return nil, fmt.Errorf("rate limit check: %w", err)
	}

	// If TTL is 0, the window just started
	if remainingTTL <= 0 {
		remainingTTL = window
	}

	result := &RateLimitResult{
		Allowed:   current <= limit,
		Current:   current,
		Limit:     limit,
		Remaining: limit - current,
		ResetAt:   time.Now().Add(remainingTTL),
	}

	if result.Remaining < 0 {
		result.Remaining = 0
	}

	return result, nil
}

// IsBackedOff checks if we're in a backoff period for an endpoint.
// Uses TTL only (no stored timestamp) to avoid clock drift issues.
func (v *ValkeyClient) IsBackedOff(ctx context.Context, key string) (bool, time.Duration, error) {
	remaining, err := v.PTTL(ctx, key)
	if err != nil {
		if err == ErrKeyNotFound {
			return false, 0, nil
		}
		return false, 0, err
	}
	if remaining > 0 {
		return true, remaining, nil
	}
	return false, 0, nil
}

// SetBackoff sets a backoff period for an endpoint.
// Uses key existence + TTL only (no stored value needed).
func (v *ValkeyClient) SetBackoff(ctx context.Context, key string, duration time.Duration) error {
	// Just set a marker key with TTL - value doesn't matter
	cmd := v.client.B().Set().Key(key).Value("1").Px(duration).Build()
	return v.client.Do(ctx, cmd).Error()
}

// ClearBackoff removes a backoff
func (v *ValkeyClient) ClearBackoff(ctx context.Context, key string) error {
	return v.Delete(ctx, key)
}

// AllowRequest checks rate limit and backoff for an endpoint/fabric combination
// Uses the key functions from keys.go
func (v *ValkeyClient) AllowRequest(ctx context.Context, endpoint, fabric string, limit int64, window time.Duration) (*RateLimitResult, error) {
	// First check backoff
	backoffKey := Backoff(endpoint, fabric)
	backedOff, remaining, err := v.IsBackedOff(ctx, backoffKey)
	if err != nil {
		return nil, err
	}
	if backedOff {
		return &RateLimitResult{
			Allowed:   false,
			Remaining: 0,
			ResetAt:   time.Now().Add(remaining),
		}, nil
	}

	// Then check rate limit
	rlKey := RateLimit(endpoint, fabric)
	return v.CheckRateLimit(ctx, rlKey, limit, window)
}

// SetRequestBackoff sets a backoff period after receiving a rate limit response from ND
func (v *ValkeyClient) SetRequestBackoff(ctx context.Context, endpoint, fabric string, duration time.Duration) error {
	backoffKey := Backoff(endpoint, fabric)
	return v.SetBackoff(ctx, backoffKey, duration)
}
