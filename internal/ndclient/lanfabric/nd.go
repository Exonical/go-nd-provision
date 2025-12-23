package lanfabric

import (
	"context"
	"fmt"

	"github.com/banglin/go-nd/internal/ndclient/common"
)

// New ND LAN Fabric API methods (stubs for future implementation)
// Uses /api/v1/lan-fabric

// GetFabricsND retrieves all fabrics from new ND API
func (s *Service) GetFabricsND(ctx context.Context) ([]FabricData, error) {
	path, err := s.client.NDLanFabricPath("fabrics")
	if err != nil {
		return nil, err
	}

	var response FabricResponse
	if err := s.client.Get(ctx, path, &response); err != nil {
		return nil, fmt.Errorf("get fabrics (nd): %w", err)
	}
	return response.Fabrics, nil
}

// GetFabricND retrieves a single fabric by ID from new ND API
func (s *Service) GetFabricND(ctx context.Context, fabricID string) (*FabricData, error) {
	if err := common.RequireNonEmpty("fabricID", fabricID); err != nil {
		return nil, err
	}

	path, err := s.client.NDLanFabricPath("fabrics", fabricID)
	if err != nil {
		return nil, err
	}

	var fabric FabricData
	if err := s.client.Get(ctx, path, &fabric); err != nil {
		return nil, fmt.Errorf("get fabric (nd, fabricID=%s): %w", fabricID, err)
	}
	return &fabric, nil
}

// FindFabricByNameND searches for a fabric by name using new ND API
func (s *Service) FindFabricByNameND(ctx context.Context, name string) (*FabricData, error) {
	if err := common.RequireNonEmpty("name", name); err != nil {
		return nil, err
	}

	fabrics, err := s.GetFabricsND(ctx)
	if err != nil {
		return nil, err
	}
	for i := range fabrics {
		if fabrics[i].Name == name {
			return &fabrics[i], nil
		}
	}
	return nil, fmt.Errorf("fabric not found (nd): %q", name)
}

// TODO: Add new ND API methods as they become available
// - GetSwitchesND
// - GetSwitchPortsND
// - GetFabricLinksND
