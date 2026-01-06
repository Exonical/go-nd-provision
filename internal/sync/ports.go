package sync

import (
	"context"
	"time"

	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncSwitchPortsResult contains the result of a port sync operation
type SyncSwitchPortsResult struct {
	Synced int // Number of ports synced
	Total  int // Total ports returned from NDFC (before filtering)
}

// SyncSwitchPorts fetches ports from NDFC and upserts them to the database.
// This is the shared implementation used by both the HTTP handler and background worker.
//
// Parameters:
//   - ctx: context for the operation
//   - db: database connection
//   - lanFabricSvc: LAN fabric service for NDFC calls
//   - switchID: local database switch ID
//   - serialNumber: switch serial number for NDFC API
//   - uplinks: map of "serial:ifName" -> true for ports to exclude (inter-switch links)
//
// Returns the number of ports synced and any error.
func SyncSwitchPorts(
	ctx context.Context,
	db *gorm.DB,
	lanFabricSvc *lanfabric.Service,
	switchID string,
	serialNumber string,
	uplinks map[string]bool,
) (*SyncSwitchPortsResult, error) {
	// Fetch ports from NDFC
	ports, err := lanFabricSvc.GetSwitchPortsNDFC(ctx, serialNumber)
	if err != nil {
		return nil, err
	}

	// Build batch of ports to upsert
	now := time.Now()
	var portsToUpsert []models.SwitchPort
	for _, p := range ports {
		// Only import Ethernet interfaces (Ethernetx/x or Ethernetx/x/x)
		if !lanfabric.IsEthernetPort(p.Name) {
			continue
		}

		// Skip uplink ports (inter-switch links)
		uplinkKey := serialNumber + ":" + p.Name
		if uplinks[uplinkKey] {
			continue
		}

		// Use deterministic ID (switch_id:port_name) for stable upserts
		portID := switchID + ":" + p.Name
		portsToUpsert = append(portsToUpsert, models.SwitchPort{
			ID:          portID,
			Name:        p.Name,
			Description: p.Description,
			Speed:       p.Speed,
			AdminState:  p.AdminState,
			IsPresent:   true,
			SwitchID:    switchID,
			LastSeenAt:  &now,
		})
	}

	if len(portsToUpsert) == 0 {
		return &SyncSwitchPortsResult{Synced: 0, Total: len(ports)}, nil
	}

	// Bulk upsert with OnConflict - single query instead of N queries
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "switch_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "speed", "admin_state", "is_present", "last_seen_at", "updated_at"}),
	}).CreateInBatches(portsToUpsert, 500).Error; err != nil {
		return nil, err
	}

	return &SyncSwitchPortsResult{Synced: len(portsToUpsert), Total: len(ports)}, nil
}
