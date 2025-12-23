package ndclient

// secFabricPath builds URL paths for the legacy NDFC security API
// Returns error if endpoints not configured (no silent fallback)
func (c *Client) secFabricPath(fabric string, parts ...string) (string, error) {
	return c.ndfcSecurityFabricPath(fabric, parts...)
}
