package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/valkey-io/valkey-go"
)

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errors.New("cache miss")

type ValkeyClient struct {
	client valkey.Client
}

var Client *ValkeyClient

func Initialize(cfg *config.ValkeyConfig) error {
	// Close existing client if re-initializing
	if Client != nil {
		Client.Close()
		Client = nil
	}

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.Address},
		Username:    cfg.Username,
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	// Ping to verify connection is working
	if err := client.Do(context.Background(), client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return fmt.Errorf("valkey ping failed: %w", err)
	}

	Client = &ValkeyClient{client: client}
	return nil
}

func (v *ValkeyClient) Close() {
	v.client.Close()
}

func (v *ValkeyClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	var cmd valkey.Completed
	if ttl > 0 {
		// Use Px (milliseconds) for sub-second precision
		cmd = v.client.B().Set().Key(key).Value(string(data)).Px(ttl).Build()
	} else {
		cmd = v.client.B().Set().Key(key).Value(string(data)).Build()
	}
	return v.client.Do(ctx, cmd).Error()
}

func (v *ValkeyClient) Get(ctx context.Context, key string, dest interface{}) error {
	cmd := v.client.B().Get().Key(key).Build()
	res := v.client.Do(ctx, cmd)

	// Check for nil (key not found) - valkey.IsValkeyNil checks for redis nil response
	if valkey.IsValkeyNil(res.Error()) {
		return ErrCacheMiss
	}
	if res.Error() != nil {
		return res.Error()
	}

	str, err := res.ToString()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(str), dest)
}

func (v *ValkeyClient) Delete(ctx context.Context, keys ...string) error {
	cmd := v.client.B().Del().Key(keys...).Build()
	return v.client.Do(ctx, cmd).Error()
}

func (v *ValkeyClient) Exists(ctx context.Context, key string) (bool, error) {
	cmd := v.client.B().Exists().Key(key).Build()
	result, err := v.client.Do(ctx, cmd).ToInt64()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (v *ValkeyClient) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return v.Set(ctx, key, value, ttl)
}

// InvalidatePattern deletes all keys matching the pattern using SCAN (not KEYS).
// SCAN is O(1) per call and won't block Valkey on large keyspaces.
func (v *ValkeyClient) InvalidatePattern(ctx context.Context, pattern string) error {
	var cursor uint64

	for {
		cmd := v.client.B().Scan().Cursor(cursor).Match(pattern).Count(1000).Build()
		res := v.client.Do(ctx, cmd)
		if res.Error() != nil {
			return res.Error()
		}

		scanEntry, err := res.AsScanEntry()
		if err != nil {
			return err
		}

		// Delete keys in batches to avoid huge DEL args
		keys := scanEntry.Elements
		for i := 0; i < len(keys); i += 500 {
			end := i + 500
			if end > len(keys) {
				end = len(keys)
			}
			if err := v.Delete(ctx, keys[i:end]...); err != nil {
				return err
			}
		}

		cursor = scanEntry.Cursor
		if cursor == 0 {
			return nil
		}
	}
}
