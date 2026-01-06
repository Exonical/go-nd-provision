package sync

import (
	"context"

	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncSwitchesResult contains the result of a switch sync operation
type SyncSwitchesResult struct {
	Synced int // Number of switches synced (leaf/border only)
	Total  int // Total switches returned from NDFC (including spines)
}

// SyncFabricSwitches fetches switches from NDFC and upserts them to the database.
// Only imports leaf, ToR, and border switches (not spines).
// Upserts by serial_number (the unique constraint) to avoid ID conflicts.
// Uses deterministic IDs based on fabric:serial for consistency.
func SyncFabricSwitches(
	ctx context.Context,
	db *gorm.DB,
	lanFabricSvc *lanfabric.Service,
	fabric *models.Fabric,
) (*SyncSwitchesResult, error) {
	switches, err := lanFabricSvc.GetSwitchesNDFC(ctx, fabric.Name)
	if err != nil {
		return nil, err
	}

	var synced int
	for _, s := range switches {
		// Only import ToR, Leaf, or Border switches (not spines)
		if !lanfabric.IsLeafOrBorder(s.SwitchRole) {
			continue
		}

		// Use deterministic ID: fabric:serial for stability across environments
		switchID := fabric.ID + ":" + s.SerialNumber

		sw := models.Switch{
			ID:           switchID,
			Name:         s.LogicalName,
			SerialNumber: s.SerialNumber,
			Model:        s.Model,
			IPAddress:    s.IPAddress,
			FabricID:     fabric.ID,
		}

		// Upsert by serial_number (unique constraint)
		// This handles cases where the same switch might have different IDs
		if err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "serial_number"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "model", "ip_address", "fabric_id", "updated_at"}),
		}).Create(&sw).Error; err != nil {
			// Log but continue - don't fail entire sync for one switch
			continue
		}
		synced++
	}

	return &SyncSwitchesResult{Synced: synced, Total: len(switches)}, nil
}
