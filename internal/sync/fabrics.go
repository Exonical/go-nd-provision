package sync

import (
	"context"

	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncFabricsResult contains the result of a fabric sync operation
type SyncFabricsResult struct {
	Synced int // Number of fabrics synced
	Total  int // Total fabrics returned from NDFC
}

// SyncFabrics fetches fabrics from NDFC and upserts them to the database.
// Upserts by name (the unique constraint) to avoid ID conflicts between
// handler and worker.
func SyncFabrics(
	ctx context.Context,
	db *gorm.DB,
	lanFabricSvc *lanfabric.Service,
) (*SyncFabricsResult, error) {
	fabrics, err := lanFabricSvc.GetFabricsNDFC(ctx)
	if err != nil {
		return nil, err
	}

	for _, f := range fabrics {
		// Use deterministic ID based on name for consistency
		fabricID := "fabric:" + f.FabricName
		fabric := models.Fabric{
			ID:   fabricID,
			Name: f.FabricName,
			Type: f.FabricType,
		}

		// Upsert by name (unique constraint) - update type if it changes
		if err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"type", "updated_at"}),
		}).Create(&fabric).Error; err != nil {
			return nil, err
		}
	}

	return &SyncFabricsResult{Synced: len(fabrics), Total: len(fabrics)}, nil
}

// EnsureFabric ensures a fabric exists in the database, creating it if needed.
// Returns the fabric record (existing or newly created).
// Uses deterministic ID based on name.
func EnsureFabric(
	ctx context.Context,
	db *gorm.DB,
	fabricName string,
	fabricType string,
) (*models.Fabric, error) {
	fabricID := "fabric:" + fabricName
	fabric := models.Fabric{
		ID:   fabricID,
		Name: fabricName,
		Type: fabricType,
	}

	// FirstOrCreate by name, setting ID and Type as defaults for new records
	if err := db.WithContext(ctx).Where("name = ?", fabricName).
		Attrs(fabric).
		FirstOrCreate(&fabric).Error; err != nil {
		return nil, err
	}

	return &fabric, nil
}
