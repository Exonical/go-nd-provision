package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// ErrLockNotAcquired is returned when a lock cannot be acquired
var ErrLockNotAcquired = errors.New("lock not acquired")

// ErrKeyNotFound is returned when a key doesn't exist
var ErrKeyNotFound = errors.New("key not found")

// SetNX sets a key only if it doesn't exist (for locks)
// Returns true if the key was set, false if it already existed
func (v *ValkeyClient) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	cmd := v.client.B().Set().Key(key).Value(value).Nx().Ex(ttl).Build()
	result := v.client.Do(ctx, cmd)
	if err := result.Error(); err != nil {
		if valkey.IsValkeyNil(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// AcquireLock attempts to acquire a distributed lock
// Returns a release function if successful, or ErrLockNotAcquired if the lock is held
func (v *ValkeyClient) AcquireLock(ctx context.Context, key string, value string, ttl time.Duration) (func() error, error) {
	acquired, err := v.SetNX(ctx, key, value, ttl)
	if err != nil {
		return nil, fmt.Errorf("acquire lock %s: %w", key, err)
	}
	if !acquired {
		return nil, ErrLockNotAcquired
	}

	// Return a release function
	release := func() error {
		return v.Delete(ctx, key)
	}
	return release, nil
}

// ExtendLock extends the TTL of an existing lock (for long operations)
func (v *ValkeyClient) ExtendLock(ctx context.Context, key string, ttl time.Duration) error {
	cmd := v.client.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()
	result, err := v.client.Do(ctx, cmd).ToInt64()
	if err != nil {
		return fmt.Errorf("extend lock %s: %w", key, err)
	}
	if result == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// SAdd adds members to a set
func (v *ValkeyClient) SAdd(ctx context.Context, key string, members ...string) error {
	cmd := v.client.B().Sadd().Key(key).Member(members...).Build()
	return v.client.Do(ctx, cmd).Error()
}

// SMembers returns all members of a set
func (v *ValkeyClient) SMembers(ctx context.Context, key string) ([]string, error) {
	cmd := v.client.B().Smembers().Key(key).Build()
	return v.client.Do(ctx, cmd).AsStrSlice()
}

// SRem removes members from a set
func (v *ValkeyClient) SRem(ctx context.Context, key string, members ...string) error {
	cmd := v.client.B().Srem().Key(key).Member(members...).Build()
	return v.client.Do(ctx, cmd).Error()
}

// InvalidateBySet deletes all keys in a set and the set itself
// Used for cache invalidation patterns
func (v *ValkeyClient) InvalidateBySet(ctx context.Context, setKey string) error {
	members, err := v.SMembers(ctx, setKey)
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil // Set doesn't exist, nothing to invalidate
		}
		return fmt.Errorf("get cache keys from %s: %w", setKey, err)
	}

	if len(members) > 0 {
		if err := v.Delete(ctx, members...); err != nil {
			return fmt.Errorf("delete cached keys: %w", err)
		}
	}

	// Delete the set itself
	return v.Delete(ctx, setKey)
}

// TrackCacheKey adds a cache key to a tracking set for later invalidation
func (v *ValkeyClient) TrackCacheKey(ctx context.Context, trackingSetKey, cacheKey string) error {
	return v.SAdd(ctx, trackingSetKey, cacheKey)
}

// SetWithTracking sets a cache value and tracks it for invalidation
func (v *ValkeyClient) SetWithTracking(ctx context.Context, key string, value interface{}, ttl time.Duration, trackingSetKey string) error {
	if err := v.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	return v.TrackCacheKey(ctx, trackingSetKey, key)
}

// LPush pushes values to the left of a list (queue)
func (v *ValkeyClient) LPush(ctx context.Context, key string, values ...string) error {
	cmd := v.client.B().Lpush().Key(key).Element(values...).Build()
	return v.client.Do(ctx, cmd).Error()
}

// RPop pops a value from the right of a list (queue)
func (v *ValkeyClient) RPop(ctx context.Context, key string) (string, error) {
	cmd := v.client.B().Rpop().Key(key).Build()
	result, err := v.client.Do(ctx, cmd).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", ErrKeyNotFound
		}
		return "", err
	}
	return result, nil
}

// BRPop blocks and pops from the right of a list with timeout
func (v *ValkeyClient) BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, string, error) {
	cmd := v.client.B().Brpop().Key(keys...).Timeout(timeout.Seconds()).Build()
	result, err := v.client.Do(ctx, cmd).AsStrSlice()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", "", ErrKeyNotFound
		}
		return "", "", err
	}
	if len(result) < 2 {
		return "", "", ErrKeyNotFound
	}
	return result[0], result[1], nil // key, value
}

// LLen returns the length of a list
func (v *ValkeyClient) LLen(ctx context.Context, key string) (int64, error) {
	cmd := v.client.B().Llen().Key(key).Build()
	return v.client.Do(ctx, cmd).ToInt64()
}

// Incr increments a counter and returns the new value
func (v *ValkeyClient) Incr(ctx context.Context, key string) (int64, error) {
	cmd := v.client.B().Incr().Key(key).Build()
	return v.client.Do(ctx, cmd).ToInt64()
}

// IncrWithTTL increments a counter and sets TTL if it's a new key.
// Returns current count and remaining TTL (for rate limiting).
func (v *ValkeyClient) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, time.Duration, error) {
	// Use a Lua script to atomically INCR, set TTL on new keys, and return both count and PTTL
	script := `
		local current = redis.call('INCR', KEYS[1])
		if current == 1 then
			redis.call('PEXPIRE', KEYS[1], ARGV[1])
		end
		local pttl = redis.call('PTTL', KEYS[1])
		return {current, pttl}
	`
	cmd := v.client.B().Eval().Script(script).Numkeys(1).Key(key).Arg(fmt.Sprintf("%d", ttl.Milliseconds())).Build()
	result, err := v.client.Do(ctx, cmd).ToArray()
	if err != nil {
		return 0, 0, err
	}
	if len(result) < 2 {
		return 0, 0, fmt.Errorf("unexpected result from rate limit script")
	}
	current, err := result[0].ToInt64()
	if err != nil {
		return 0, 0, err
	}
	pttlMs, err := result[1].ToInt64()
	if err != nil {
		return 0, 0, err
	}
	if pttlMs < 0 {
		pttlMs = 0
	}
	return current, time.Duration(pttlMs) * time.Millisecond, nil
}

// GetInt64 gets an integer value
func (v *ValkeyClient) GetInt64(ctx context.Context, key string) (int64, error) {
	cmd := v.client.B().Get().Key(key).Build()
	result, err := v.client.Do(ctx, cmd).ToInt64()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return 0, ErrKeyNotFound
		}
		return 0, err
	}
	return result, nil
}

// SetInt64 sets an integer value with TTL (millisecond precision)
func (v *ValkeyClient) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	cmd := v.client.B().Set().Key(key).Value(fmt.Sprintf("%d", value)).Px(ttl).Build()
	return v.client.Do(ctx, cmd).Error()
}

// PTTL returns the remaining TTL of a key in milliseconds
func (v *ValkeyClient) PTTL(ctx context.Context, key string) (time.Duration, error) {
	cmd := v.client.B().Pttl().Key(key).Build()
	ms, err := v.client.Do(ctx, cmd).ToInt64()
	if err != nil {
		return 0, err
	}
	// -2 = key doesn't exist, -1 = key exists but no TTL
	if ms < 0 {
		return 0, ErrKeyNotFound
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// Expire sets a TTL on an existing key
func (v *ValkeyClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	cmd := v.client.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()
	return v.client.Do(ctx, cmd).Error()
}

// GetString gets a raw string value (no JSON unmarshal)
func (v *ValkeyClient) GetString(ctx context.Context, key string) (string, error) {
	cmd := v.client.B().Get().Key(key).Build()
	result, err := v.client.Do(ctx, cmd).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", ErrKeyNotFound
		}
		return "", err
	}
	return result, nil
}

// SetString sets a raw string value with TTL
func (v *ValkeyClient) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	cmd := v.client.B().Set().Key(key).Value(value).Ex(ttl).Build()
	return v.client.Do(ctx, cmd).Error()
}
