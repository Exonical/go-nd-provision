package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/ndclient"
	"go.uber.org/zap"
)

// DeployBatcher batches ConfigDeploy requests to avoid overwhelming NDFC.
// Uses Valkey for distributed coordination across multiple service instances.
//
// Behavior:
//   - First request records batch start time in Valkey and starts polling
//   - Subsequent requests update the "last request" timestamp
//   - After debounce period OR max wait time (whichever comes first), one instance acquires lock and deploys
//   - All waiters poll for completion and receive the same result
//   - If new requests arrive during deploy, they form a NEW batch that waits for the lock
//   - This ensures sequential deploys: batch1 deploys -> batch2 deploys -> etc.
//
// Valkey keys used (per fabric):
//   - deploy:batch:{fabric}:start    - Unix timestamp of first request in batch (also serves as batch ID)
//   - deploy:batch:{fabric}:last     - Unix timestamp of last request in batch
//   - deploy:batch:{fabric}:lock     - Lock for executing deploy (only one instance)
//   - deploy:batch:{fabric}:result:{batchID} - Result of deploy ("ok" or error message)
type DeployBatcher struct {
	ndClient     *ndclient.Client
	cache        *cache.ValkeyClient
	debounceTime time.Duration
	maxWaitTime  time.Duration

	// Local waiters for this instance (to notify when deploy completes)
	mu      sync.Mutex
	waiters map[string][]chan error // fabricName -> local waiters

	// Track which fabrics have a result watcher running (to avoid spawning duplicates)
	watcherMu sync.Mutex
	watchers  map[string]bool // fabricName -> has active watcher
}

// NewDeployBatcher creates a new deploy batcher.
// debounceTime: how long to wait after the last request before deploying (e.g., 5s)
// maxWaitTime: maximum time to wait before forcing deploy regardless of new requests (e.g., 20s)
func NewDeployBatcher(ndClient *ndclient.Client, debounceTime, maxWaitTime time.Duration) *DeployBatcher {
	return &DeployBatcher{
		ndClient:     ndClient,
		cache:        cache.Client,
		debounceTime: debounceTime,
		maxWaitTime:  maxWaitTime,
		waiters:      make(map[string][]chan error),
		watchers:     make(map[string]bool),
	}
}

// Valkey key helpers
// keyStart stores the batch ID (timestamp of first request) - used to identify the batch
func (b *DeployBatcher) keyStart(fabric string) string {
	return fmt.Sprintf("deploy:batch:%s:start", fabric)
}
func (b *DeployBatcher) keyLast(fabric string) string {
	return fmt.Sprintf("deploy:batch:%s:last", fabric)
}
func (b *DeployBatcher) keyLock(fabric string) string {
	return fmt.Sprintf("deploy:batch:%s:lock", fabric)
}

// keyResult includes the batch ID to prevent cross-batch result confusion
func (b *DeployBatcher) keyResult(fabric, batchID string) string {
	return fmt.Sprintf("deploy:batch:%s:result:%s", fabric, batchID)
}

// RequestDeploy queues a deploy request for the given fabric.
// Uses Valkey for distributed coordination - works across multiple instances.
// Returns when the deploy completes (or fails).
func (b *DeployBatcher) RequestDeploy(ctx context.Context, fabricName string) error {
	if b.cache == nil {
		// Fallback: no Valkey, deploy immediately
		logger.Warn("DeployBatcher: Valkey not available, deploying immediately",
			zap.String("fabric", fabricName))
		return b.ndClient.ConfigDeploy(ctx, fabricName, nil)
	}

	resultCh := make(chan error, 1)
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	keyStart := b.keyStart(fabricName)
	keyLast := b.keyLast(fabricName)
	ttl := b.maxWaitTime + 10*time.Second

	// Register local waiter immediately (before any blocking operations)
	b.mu.Lock()
	b.waiters[fabricName] = append(b.waiters[fabricName], resultCh)
	b.mu.Unlock()

	// Try to set batch start time (only succeeds if no batch exists)
	// The start value (nowStr) serves as the batch ID
	isFirst, err := b.cache.SetNX(ctx, keyStart, nowStr, ttl)
	if err != nil {
		b.removeWaiter(fabricName, resultCh)
		return fmt.Errorf("deploy batch: set start time: %w", err)
	}

	// Get the actual batch ID (may be different if we joined existing batch)
	batchID := nowStr
	if !isFirst {
		// Read the existing batch's start time to get its ID
		existingBatchID, err := b.cache.GetString(ctx, keyStart)
		if err == nil && existingBatchID != "" {
			batchID = existingBatchID
		}
	}

	// Update last request time (raw string, not JSON)
	if err := b.cache.SetString(ctx, keyLast, nowStr, ttl); err != nil {
		if isFirst {
			// Cleanup start key if we created it but failed to set last
			_ = b.cache.Delete(ctx, keyStart)
		}
		b.removeWaiter(fabricName, resultCh)
		return fmt.Errorf("deploy batch: set last time: %w", err)
	}

	if isFirst {
		logger.Debug("Deploy batch started (first request)",
			zap.String("fabric", fabricName),
			zap.String("batchID", batchID))
		// Start the deploy coordinator goroutine with bounded context
		go b.coordinateDeploy(fabricName, batchID)
	} else {
		logger.Debug("Deploy request added to batch",
			zap.String("fabric", fabricName),
			zap.String("batchID", batchID))
		// Ensure there's a result watcher for this instance (joiners need to be notified too)
		b.ensureResultWatcher(fabricName, batchID)
	}

	// Wait for result or context cancellation
	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		b.removeWaiter(fabricName, resultCh)
		return ctx.Err()
	}
}

// coordinateDeploy polls Valkey and executes deploy when conditions are met
// batchID is the timestamp of the first request, used to namespace the result key
func (b *DeployBatcher) coordinateDeploy(fabricName, batchID string) {
	// Bounded context to prevent runaway goroutines
	ctx, cancel := context.WithTimeout(context.Background(), b.maxWaitTime+2*time.Minute)
	defer cancel()

	keyStart := b.keyStart(fabricName)
	keyLast := b.keyLast(fabricName)
	keyLock := b.keyLock(fabricName)
	keyResult := b.keyResult(fabricName, batchID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Warn("Deploy coordinator timed out",
				zap.String("fabric", fabricName),
				zap.String("batchID", batchID))
			// Write shared result so other instances don't hang
			// Use fresh context since ctx is already cancelled
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = b.cache.SetString(cleanupCtx, keyResult, "coordinator timeout", 30*time.Second)
			cleanupCancel()
			b.notifyWaiters(fabricName, "coordinator timeout")
			return
		case <-ticker.C:
		}

		// Check if we should deploy
		shouldDeploy, err := b.shouldDeploy(ctx, keyStart, keyLast)
		if err != nil {
			logger.Warn("Deploy batch: check failed",
				zap.String("fabric", fabricName),
				zap.Error(err))
			continue
		}

		if !shouldDeploy {
			// Check if another instance already deployed (result exists for THIS batch)
			result, err := b.cache.GetString(ctx, keyResult)
			if err == nil && result != "" {
				// Result available - notify waiters and exit
				b.notifyWaiters(fabricName, result)
				return
			}
			// Distinguish key not found (normal) vs other errors (log them)
			if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
				logger.Debug("Deploy batch: result check error",
					zap.String("fabric", fabricName),
					zap.Error(err))
			}
			continue
		}

		// Try to acquire deploy lock (30 minute TTL for slow NDFC deploys)
		release, err := b.cache.AcquireLock(ctx, keyLock, "deploying", 30*time.Minute)
		if errors.Is(err, cache.ErrLockNotAcquired) {
			// Another instance is deploying - wait for result
			continue
		}
		if err != nil {
			logger.Warn("Deploy batch: lock failed",
				zap.String("fabric", fabricName),
				zap.Error(err))
			continue
		}

		// We have the lock - execute deploy
		logger.Info("Executing batched deploy",
			zap.String("fabric", fabricName),
			zap.String("batchID", batchID))

		deployErr := b.ndClient.ConfigDeploy(ctx, fabricName, nil)

		// Store result (raw string, not JSON)
		result := "ok"
		if deployErr != nil {
			result = deployErr.Error()
			logger.Warn("Batched deploy failed",
				zap.String("fabric", fabricName),
				zap.Error(deployErr))
		} else {
			logger.Info("Batched deploy succeeded",
				zap.String("fabric", fabricName))
		}

		// Store result for other instances to read (raw string)
		if err := b.cache.SetString(ctx, keyResult, result, 30*time.Second); err != nil {
			logger.Error("Deploy batch: failed to write result",
				zap.String("fabric", fabricName),
				zap.String("batchID", batchID),
				zap.Error(err))
			// Don't delete batch keys if result write failed - let them expire
			// so another coordinator can potentially retry
			_ = release()
			b.notifyWaiters(fabricName, result)
			return
		}

		// Cleanup: only delete keyLast, let keyStart expire naturally
		// This prevents race where joiner reads keyStart after we delete it
		// but before they can read the result
		currentBatchID, err := b.cache.GetString(ctx, keyStart)
		if err == nil && currentBatchID == batchID {
			_ = b.cache.Delete(ctx, keyLast)
		}
		_ = release()

		// Notify local waiters
		b.notifyWaiters(fabricName, result)
		return
	}
}

// shouldDeploy checks if debounce or max wait conditions are met
func (b *DeployBatcher) shouldDeploy(ctx context.Context, keyStart, keyLast string) (bool, error) {
	now := time.Now().UnixMilli()

	// Get start time (raw string, not JSON)
	startStr, err := b.cache.GetString(ctx, keyStart)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return false, nil // Batch was already processed
		}
		return false, err
	}
	startMs, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		// Corrupted value - treat as not ready (will eventually expire)
		return false, fmt.Errorf("parse start time: %w", err)
	}

	// Get last request time (raw string, not JSON)
	lastStr, err := b.cache.GetString(ctx, keyLast)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	lastMs, err := strconv.ParseInt(lastStr, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse last time: %w", err)
	}

	// Check max wait
	elapsed := time.Duration(now-startMs) * time.Millisecond
	if elapsed >= b.maxWaitTime {
		logger.Debug("Deploy batch: max wait reached",
			zap.Duration("elapsed", elapsed),
			zap.Duration("maxWait", b.maxWaitTime))
		return true, nil
	}

	// Check debounce
	sinceLast := time.Duration(now-lastMs) * time.Millisecond
	if sinceLast >= b.debounceTime {
		logger.Debug("Deploy batch: debounce complete",
			zap.Duration("sinceLast", sinceLast),
			zap.Duration("debounce", b.debounceTime))
		return true, nil
	}

	return false, nil
}

// notifyWaiters notifies all local waiters with the result
func (b *DeployBatcher) notifyWaiters(fabricName, result string) {
	b.mu.Lock()
	waiters := b.waiters[fabricName]
	delete(b.waiters, fabricName)
	b.mu.Unlock()

	var err error
	if result != "ok" {
		err = errors.New(result)
	}

	for _, ch := range waiters {
		select {
		case ch <- err:
		default:
		}
	}
}

// removeWaiter removes a specific waiter (for context cancellation)
func (b *DeployBatcher) removeWaiter(fabricName string, ch chan error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	waiters := b.waiters[fabricName]
	for i, w := range waiters {
		if w == ch {
			b.waiters[fabricName] = append(waiters[:i], waiters[i+1:]...)
			return
		}
	}
}

// ensureResultWatcher starts a result watcher goroutine for this fabric if one isn't already running.
// This is needed for joiner instances that didn't start the coordinator but have local waiters.
func (b *DeployBatcher) ensureResultWatcher(fabricName, batchID string) {
	b.watcherMu.Lock()
	if b.watchers[fabricName] {
		b.watcherMu.Unlock()
		return // Already watching
	}
	b.watchers[fabricName] = true
	b.watcherMu.Unlock()

	go b.watchForResult(fabricName, batchID)
}

// watchForResult polls for the batch result and notifies local waiters when it appears.
// This handles the case where another instance executes the deploy.
func (b *DeployBatcher) watchForResult(fabricName, batchID string) {
	ctx, cancel := context.WithTimeout(context.Background(), b.maxWaitTime+2*time.Minute)
	defer cancel()
	defer func() {
		b.watcherMu.Lock()
		delete(b.watchers, fabricName)
		b.watcherMu.Unlock()
	}()

	keyResult := b.keyResult(fabricName, batchID)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout - notify waiters with error
			b.notifyWaiters(fabricName, "result watcher timeout")
			return
		case <-ticker.C:
			// Check if result exists
			result, err := b.cache.GetString(ctx, keyResult)
			if err == nil && result != "" {
				// Result available - notify local waiters
				b.notifyWaiters(fabricName, result)
				return
			}
			// Also check if we still have local waiters - if not, stop watching
			b.mu.Lock()
			hasWaiters := len(b.waiters[fabricName]) > 0
			b.mu.Unlock()
			if !hasWaiters {
				return
			}
		}
	}
}

// PendingCount returns the number of pending local deploy requests for a fabric (for testing)
func (b *DeployBatcher) PendingCount(fabricName string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.waiters[fabricName])
}
