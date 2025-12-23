package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

// Worker handles background synchronization of NDFC data
type Worker struct {
	ndClient   *ndclient.Client
	interval   time.Duration
	fabricName string

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	started atomic.Bool // Prevents double Start()
}

// NewWorker creates a new sync worker
func NewWorker(ndClient *ndclient.Client, cfg *config.NexusDashboardConfig) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		ndClient:   ndClient,
		interval:   time.Duration(cfg.SyncIntervalHours) * time.Hour,
		fabricName: cfg.ComputeFabricName,
		ctx:        ctx,
		cancel:     cancel,
	}
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

func (w *Worker) syncAll() {
	// Prevent overlapping syncs
	if !w.running.CompareAndSwap(false, true) {
		logger.Warn("NDFC sync skipped: previous run still active")
		return
	}
	defer w.running.Store(false)

	if w.fabricName == "" {
		logger.Warn("NDFC sync skipped: no fabric name configured")
		return
	}

	// Use parent context with timeout (15m for large fabrics with many switches)
	ctx, cancel := context.WithTimeout(w.ctx, 15*time.Minute)
	defer cancel()

	logger.Info("Starting NDFC sync", zap.String("fabric", w.fabricName))
	start := time.Now()

	// Sync switches
	switchStart := time.Now()
	switchCount, err := w.syncSwitches(ctx)
	switchDuration := time.Since(switchStart)
	if err != nil {
		logger.Error("Failed to sync switches", zap.Error(err), zap.Duration("duration", switchDuration))
		return
	}

	// Sync ports for each switch
	portStart := time.Now()
	portCount, portErrors, err := w.syncPorts(ctx)
	portDuration := time.Since(portStart)
	if err != nil {
		logger.Error("Failed to sync ports", zap.Error(err), zap.Duration("duration", portDuration))
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

	// Atomic fabric upsert - unique constraint on name prevents race conditions
	var fabric models.Fabric
	if err := db.Where("name = ?", w.fabricName).
		Attrs(models.Fabric{ID: uuid.New().String(), Type: "VXLAN"}).
		FirstOrCreate(&fabric).Error; err != nil {
		return 0, fmt.Errorf("upsert fabric: %w", err)
	}

	switches, err := w.ndClient.LANFabric().GetSwitchesNDFC(ctx, w.fabricName)
	if err != nil {
		return 0, fmt.Errorf("get switches from NDFC: %w", err)
	}

	// Only import ToR, Leaf, or Border switches (not spines)
	var imported int
	for _, s := range switches {
		if !lanfabric.IsLeafOrBorder(s.SwitchRole) {
			continue
		}

		// Use fabric:serial as stable ID (avoids NDFC ID collisions across fabrics)
		switchID := fabric.ID + ":" + s.SerialNumber

		sw := models.Switch{
			ID:           switchID,
			Name:         s.LogicalName,
			SerialNumber: s.SerialNumber,
			Model:        s.Model,
			IPAddress:    s.IPAddress,
			FabricID:     fabric.ID,
		}

		// Upsert switch
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "model", "ip_address", "updated_at"}),
		}).Create(&sw).Error; err != nil {
			logger.Warn("Failed to upsert switch",
				zap.String("switch", s.LogicalName),
				zap.Error(err),
			)
			continue
		}
		imported++
	}

	return imported, nil
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

	// Get uplink ports to exclude (inter-switch links)
	uplinks, err := w.ndClient.LANFabric().GetUplinkPortsNDFC(ctx, w.fabricName)
	if err != nil {
		logger.Warn("Failed to get uplink ports, continuing without filter", zap.Error(err))
		uplinks = make(map[string]bool)
	}

	now := time.Now()
	var totalPorts int
	var totalErrors int

	for _, sw := range switches {
		if sw.SerialNumber == "" {
			continue
		}

		// Per-switch timeout to prevent one slow switch from blocking the entire sync
		swCtx, swCancel := context.WithTimeout(ctx, 45*time.Second)
		ports, err := w.ndClient.LANFabric().GetSwitchPortsNDFC(swCtx, sw.SerialNumber)
		swCancel()
		if err != nil {
			logger.Warn("Failed to fetch ports for switch",
				zap.String("switch", sw.Name),
				zap.Error(err),
			)
			totalErrors++
			continue
		}

		// Build batch of ports to upsert
		var portsToUpsert []models.SwitchPort
		for _, p := range ports {
			// Normalize and filter
			name := strings.TrimSpace(p.Name)
			if !isEthernetPort(name) {
				continue
			}

			// Skip uplink ports (inter-switch links)
			uplinkKey := sw.SerialNumber + ":" + name
			if uplinks[uplinkKey] {
				continue
			}

			// Use deterministic ID (switch_id:port_name) for stable upserts
			portID := sw.ID + ":" + name
			portsToUpsert = append(portsToUpsert, models.SwitchPort{
				ID:          portID,
				Name:        name,
				Description: p.Description,
				Speed:       p.Speed,
				Status:      p.AdminState,
				SwitchID:    sw.ID,
				LastSeenAt:  &now,
			})
		}

		if len(portsToUpsert) == 0 {
			continue
		}

		// Bulk upsert with OnConflict
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "switch_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"description", "speed", "status", "last_seen_at", "updated_at"}),
		}).CreateInBatches(portsToUpsert, 500).Error; err != nil {
			logger.Warn("Failed to upsert ports for switch",
				zap.String("switch", sw.Name),
				zap.Int("count", len(portsToUpsert)),
				zap.Error(err),
			)
			totalErrors++
			continue
		}

		totalPorts += len(portsToUpsert)
	}

	// Mark stale ports as absent (not seen in this sync run)
	// This keeps inventory accurate when ports are removed from switches
	staleThreshold := now.Add(-24 * time.Hour) // Ports not seen for 24h are marked absent
	var switchIDs []string
	for _, sw := range switches {
		switchIDs = append(switchIDs, sw.ID)
	}
	if len(switchIDs) > 0 {
		result := db.Model(&models.SwitchPort{}).
			Where("switch_id IN ?", switchIDs).
			Where("last_seen_at < ?", staleThreshold).
			Where("status != ?", "absent").
			Update("status", "absent")
		if result.Error != nil {
			logger.Warn("Failed to mark stale ports as absent", zap.Error(result.Error))
		} else if result.RowsAffected > 0 {
			logger.Info("Marked stale ports as absent", zap.Int64("count", result.RowsAffected))
		}
	}

	return totalPorts, totalErrors, nil
}

// isEthernetPort checks if the port name is an Ethernet interface
// Handles both "Ethernet1/1" and "Eth1/1" formats
func isEthernetPort(name string) bool {
	return strings.HasPrefix(name, "Ethernet") || strings.HasPrefix(name, "Eth")
}
