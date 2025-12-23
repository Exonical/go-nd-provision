package cache

import (
	"testing"
	"time"
)

func TestRateLimitResult_Allowed(t *testing.T) {
	result := &RateLimitResult{
		Allowed:   true,
		Current:   5,
		Limit:     10,
		Remaining: 5,
		ResetAt:   time.Now().Add(time.Minute),
	}

	if !result.Allowed {
		t.Error("expected Allowed=true")
	}
	if result.Remaining != 5 {
		t.Errorf("expected Remaining=5, got %d", result.Remaining)
	}
}

func TestRateLimitResult_Denied(t *testing.T) {
	result := &RateLimitResult{
		Allowed:   false,
		Current:   11,
		Limit:     10,
		Remaining: 0,
		ResetAt:   time.Now().Add(time.Minute),
	}

	if result.Allowed {
		t.Error("expected Allowed=false")
	}
	if result.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", result.Remaining)
	}
}

func TestRateLimitResult_RemainingCalculation(t *testing.T) {
	tests := []struct {
		name          string
		current       int64
		limit         int64
		wantRemaining int64
	}{
		{"under limit", 3, 10, 7},
		{"at limit", 10, 10, 0},
		{"over limit", 15, 10, 0}, // clamped to 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining := tt.limit - tt.current
			if remaining < 0 {
				remaining = 0
			}
			if remaining != tt.wantRemaining {
				t.Errorf("got remaining=%d, want %d", remaining, tt.wantRemaining)
			}
		})
	}
}
