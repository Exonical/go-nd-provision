package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"go.uber.org/zap"
)

// Worker handles background synchronization of NDFC data
type Worker struct {
	ndClient   *ndclient.Client
	interval   time.Duration
	fabricName string
	instanceID string // Unique identifier for this worker instance (for debugging)

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	started atomic.Bool // Prevents double Start()
}

// NewWorker creates a new sync worker
func NewWorker(ndClient *ndclient.Client, cfg *config.NexusDashboardConfig) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	// Generate instance ID from hostname + pid for debugging multi-instance issues
	instanceID := generateInstanceID()
	return &Worker{
		ndClient:   ndClient,
		interval:   time.Duration(cfg.SyncIntervalHours) * time.Hour,
		fabricName: cfg.ComputeFabricName,
		instanceID: instanceID,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// generateInstanceID creates a unique identifier for this worker instance
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

// Start begins the background sync routine
func (w *Worker) Start() {
	if w.interval <= 0 {
		logger.Info("NDFC sync disabled (interval = 0)")
		return
	}

	// Prevent double Start() - only first call proceeds
	if !w.started.CompareAndSwap(false, true) {
		logger.Warn("NDFC sync worker already started, ignoring duplicate Start()")
		return
	}

	logger.Info("Starting NDFC sync worker",
		zap.Duration("interval", w.interval),
		zap.String("fabric", w.fabricName),
	)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		// Initial sync (no sleep, just run)
		w.syncAll()

		for {
			select {
			case <-ticker.C:
				w.syncAll()
			case <-w.ctx.Done():
				logger.Info("NDFC sync worker stopped")
				return
			}
		}
	}()
}

// Stop stops the background sync routine and waits for completion
func (w *Worker) Stop() {
	w.cancel()
	w.wg.Wait()
}

// Sync lock and cache key formats and TTLs
const (
	syncKeyPrefix    = "sync:ndfc:"
	syncLockTTL      = 30 * time.Minute // Must exceed ctx timeout (15m) + buffer
	uplinksCacheTTL  = 30 * time.Minute
	statusTTL        = 24 * time.Hour
	cooldownDuration = 5 * time.Minute
	cacheOpTimeout   = 2 * time.Second
)

// syncKeyFor builds a Valkey key for the given fabric and suffix
func (w *Worker) syncKeyFor(suffix string) string {
	return syncKeyPrefix + w.fabricName + ":" + suffix
}

func (w *Worker) syncAll() {
	// Prevent overlapping syncs (local instance)
	if !w.running.CompareAndSwap(false, true) {
		logger.Warn("NDFC sync skipped: previous run still active")
		return
	}
	defer w.running.Store(false)

	if w.fabricName == "" {
		logger.Warn("NDFC sync skipped: no fabric name configured")
		return
	}

	// Check cooldown (skip if we recently had failures)
	if w.isOnCooldown() {
		logger.Debug("NDFC sync skipped: on cooldown after recent failures",
			zap.String("fabric", w.fabricName))
		return
	}

	// Distributed lock to prevent multiple instances from syncing simultaneously
	// Use bounded context for lock acquisition to avoid hangs
	lockKey := w.syncKeyFor("lock")
	valkeyClient := cache.Client
	var release func() error
	if valkeyClient != nil {
		lockCtx, lockCancel := context.WithTimeout(w.ctx, cacheOpTimeout)
		var err error
		release, err = valkeyClient.AcquireLock(lockCtx, lockKey, "sync-worker:"+w.instanceID, syncLockTTL)
		lockCancel()

		if err != nil {
			if errors.Is(err, cache.ErrLockNotAcquired) {
				logger.Debug("NDFC sync skipped: another instance holds the lock",
					zap.String("fabric", w.fabricName))
				return
			}
			// For any other error (network issues, timeouts), skip to avoid duplicate syncs
			// Only proceed without lock if Valkey is completely unavailable (Client == nil)
			logger.Warn("NDFC sync skipped: lock acquisition failed",
				zap.String("fabric", w.fabricName),
				zap.Error(err))
			return
		}
	}

	// Use parent context with timeout (15m for large fabrics with many switches)
	ctx, cancel := context.WithTimeout(w.ctx, 15*time.Minute)
	defer cancel()

	// Track sync state for status updates
	start := time.Now()
	var syncErr error
	var portErrors int
	var portCount int

	// Set in_progress flag and ensure cleanup on exit
	w.setInProgress(true)
	defer func() {
		w.setInProgress(false)
		w.updateSyncStatus(time.Since(start), portErrors, syncErr)
		w.setFinishStatus(syncErr)
		if release != nil {
			_ = release()
		}
	}()

	// Record last_run_ts immediately for "is it alive?" visibility
	w.setLastRunTS()

	logger.Info("Starting NDFC sync", zap.String("fabric", w.fabricName))

	// Sync switches
	switchStart := time.Now()
	switchCount, err := w.syncSwitches(ctx)
	switchDuration := time.Since(switchStart)
	if err != nil {
		logger.Error("Failed to sync switches", zap.Error(err), zap.Duration("duration", switchDuration))
		syncErr = err
		w.setCooldown() // Set cooldown on failure
		return
	}

	// Sync ports for each switch
	portStart := time.Now()
	portCount, portErrors, err = w.syncPorts(ctx)
	portDuration := time.Since(portStart)
	if err != nil {
		logger.Error("Failed to sync ports", zap.Error(err), zap.Duration("duration", portDuration))
		syncErr = err
		w.setCooldown() // Set cooldown on failure
	}

	logger.Info("NDFC sync completed",
		zap.String("fabric", w.fabricName),
		zap.Int("switches", switchCount),
		zap.Int("ports", portCount),
		zap.Int("port_errors", portErrors),
		zap.Duration("switch_duration", switchDuration),
		zap.Duration("port_duration", portDuration),
		zap.Duration("total_duration", time.Since(start)),
	)
}

func (w *Worker) syncSwitches(ctx context.Context) (int, error) {
	db := database.DB.WithContext(ctx)

	// Ensure fabric exists using shared helper
	fabric, err := EnsureFabric(ctx, db, w.fabricName, "VXLAN")
	if err != nil {
		return 0, fmt.Errorf("ensure fabric: %w", err)
	}

	// Sync switches using shared helper
	result, err := SyncFabricSwitches(ctx, db, w.ndClient.LANFabric(), fabric)
	if err != nil {
		return 0, fmt.Errorf("sync switches: %w", err)
	}

	return result.Synced, nil
}

func (w *Worker) syncPorts(ctx context.Context) (int, int, error) {
	db := database.DB.WithContext(ctx)

	// Get fabric
	var fabric models.Fabric
	if err := db.Where("name = ?", w.fabricName).First(&fabric).Error; err != nil {
		return 0, 0, fmt.Errorf("fabric not found: %w", err)
	}

	// Get all switches for the fabric
	var switches []models.Switch
	if err := db.Where("fabric_id = ?", fabric.ID).Find(&switches).Error; err != nil {
		return 0, 0, fmt.Errorf("get switches: %w", err)
	}

	// Get uplink ports to exclude (inter-switch links) - cached for 30 minutes
	uplinks := w.getUplinksWithCache(ctx)

	now := time.Now()
	var totalPorts int
	var totalErrors int

	for _, sw := range switches {
		if sw.SerialNumber == "" {
			continue
		}

		// Per-switch timeout to prevent one slow switch from blocking the entire sync
		swCtx, swCancel := context.WithTimeout(ctx, 45*time.Second)
		result, err := SyncSwitchPorts(swCtx, db, w.ndClient.LANFabric(), sw.ID, sw.SerialNumber, uplinks)
		swCancel()
		if err != nil {
			logger.Warn("Failed to sync ports for switch",
				zap.String("switch", sw.Name),
				zap.Error(err),
			)
			totalErrors++
			continue
		}

		totalPorts += result.Synced
	}

	// Mark stale ports as not present (not seen in recent sync)
	// This keeps inventory accurate when ports are removed from switches
	staleThreshold := now.Add(-24 * time.Hour) // Ports not seen for 24h are marked not present
	var switchIDs []string
	for _, sw := range switches {
		switchIDs = append(switchIDs, sw.ID)
	}
	if len(switchIDs) > 0 {
		result := db.Model(&models.SwitchPort{}).
			Where("switch_id IN ?", switchIDs).
			Where("last_seen_at < ?", staleThreshold).
			Where("is_present = ?", true).
			Update("is_present", false)
		if result.Error != nil {
			logger.Warn("Failed to mark stale ports as not present", zap.Error(result.Error))
		} else if result.RowsAffected > 0 {
			logger.Info("Marked stale ports as not present", zap.Int64("count", result.RowsAffected))
		}
	}

	return totalPorts, totalErrors, nil
}

// isOnCooldown checks if we're in a cooldown period after recent failures
func (w *Worker) isOnCooldown() bool {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	cooldownKey := w.syncKeyFor("cooldown_until")
	tsStr, err := valkeyClient.GetString(ctx, cooldownKey)
	if err != nil {
		return false // No cooldown or error reading
	}

	cooldownUntil, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}

	return time.Now().Unix() < cooldownUntil
}

// setCooldown sets a cooldown period after failures
func (w *Worker) setCooldown() {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	cooldownKey := w.syncKeyFor("cooldown_until")
	cooldownUntil := time.Now().Add(cooldownDuration).Unix()
	// TTL slightly longer than cooldown to handle clock drift
	_ = valkeyClient.SetString(ctx, cooldownKey, strconv.FormatInt(cooldownUntil, 10), cooldownDuration+time.Minute)
}

// setInProgress sets or clears the in_progress flag
func (w *Worker) setInProgress(inProgress bool) {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	key := w.syncKeyFor("in_progress")
	if inProgress {
		// Include instanceID and timestamp for debugging multi-instance issues
		val := fmt.Sprintf("%s:%d", w.instanceID, time.Now().Unix())
		_ = valkeyClient.SetString(ctx, key, val, syncLockTTL)
	} else {
		// Write "0" instead of delete - more robust if delete fails
		_ = valkeyClient.SetString(ctx, key, "0", 10*time.Minute)
	}
}

// setLastRunTS records when sync started for "is it alive?" visibility
func (w *Worker) setLastRunTS() {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_run_ts"), strconv.FormatInt(time.Now().Unix(), 10), statusTTL)
}

// setFinishStatus records completion time and status for observability
func (w *Worker) setFinishStatus(syncErr error) {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	// Always write finish timestamp
	_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_finish_ts"), strconv.FormatInt(time.Now().Unix(), 10), statusTTL)

	// Write status: "ok" or "error"
	status := "ok"
	if syncErr != nil {
		status = "error"
	}
	_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_status"), status, statusTTL)
}

// updateSyncStatus stores sync status in Valkey for observability/health endpoints
func (w *Worker) updateSyncStatus(duration time.Duration, errorCount int, syncErr error) {
	valkeyClient := cache.Client
	if valkeyClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, cacheOpTimeout)
	defer cancel()

	// Store last success timestamp and clear last_error on success
	if syncErr == nil {
		_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_success_ts"), strconv.FormatInt(time.Now().Unix(), 10), statusTTL)
		_ = valkeyClient.Delete(ctx, w.syncKeyFor("last_error")) // Clear stale error
	} else {
		_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_error"), syncErr.Error(), statusTTL)
	}

	// Store duration in milliseconds
	_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_duration_ms"), strconv.FormatInt(duration.Milliseconds(), 10), statusTTL)

	// Store error count
	_ = valkeyClient.SetString(ctx, w.syncKeyFor("last_error_count"), strconv.Itoa(errorCount), statusTTL)
}

// getUplinksWithCache returns uplink ports, using Valkey cache when available
func (w *Worker) getUplinksWithCache(ctx context.Context) map[string]bool {
	return GetUplinksWithCache(ctx, w.ndClient.LANFabric(), w.fabricName, cache.Client)
}
