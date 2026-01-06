package sync

import (
	"context"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
)

// Note: uplinksCacheTTL and cacheOpTimeout are defined in worker.go

// GetUplinksWithCache returns uplink ports for a fabric, using Valkey cache when available.
// This is the shared implementation used by both the HTTP handler and background worker.
//
// Parameters:
//   - ctx: context for the operation
//   - lanFabricSvc: LAN fabric service for NDFC calls
//   - fabricName: fabric name for NDFC API and cache key
//   - cacheClient: optional Valkey client (pass nil to skip caching)
//
// Returns a map of "serial:ifName" -> true for all uplink ports.
// On error, returns an empty map (graceful degradation).
func GetUplinksWithCache(
	ctx context.Context,
	lanFabricSvc *lanfabric.Service,
	fabricName string,
	cacheClient *cache.ValkeyClient,
) map[string]bool {
	cacheKey := "cache:v1:uplinks:" + fabricName

	// Try cache first with bounded context
	if cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, cacheOpTimeout)
		var cached map[string]bool
		err := cacheClient.Get(cacheCtx, cacheKey, &cached)
		cancel()
		if err == nil && cached != nil {
			return cached
		}
	}

	// Fetch from NDFC
	uplinks, err := lanFabricSvc.GetUplinkPortsNDFC(ctx, fabricName)
	if err != nil {
		// Graceful degradation - return empty map on error
		return make(map[string]bool)
	}

	// Cache the result with bounded context
	if cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, cacheOpTimeout)
		_ = cacheClient.Set(cacheCtx, cacheKey, uplinks, uplinksCacheTTL)
		cancel()
	}

	return uplinks
}
