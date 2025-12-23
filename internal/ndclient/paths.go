package ndclient

import "net/url"

// Legacy NDFC path builders

// ndfcSecurityFabricPath builds a path for the legacy NDFC security API
// Example: /appcenter/cisco/ndfc/api/v1/security/fabrics/{fabric}/groups
func (c *Client) ndfcSecurityFabricPath(fabric string, parts ...string) (string, error) {
	base, err := c.endpoints.base(APINDFCSecurityV1)
	if err != nil {
		return "", err
	}
	allParts := append([]string{"fabrics", fabric}, parts...)
	return joinPath(base, allParts...), nil
}

// ndfcLanFabricPath builds a path for the legacy NDFC LAN fabric API
// Example: /appcenter/cisco/ndfc/api/v1/lan-fabric/fabrics/{fabricID}/switches
func (c *Client) ndfcLanFabricPath(parts ...string) (string, error) {
	base, err := c.endpoints.base(APINDFCLANFabricV1)
	if err != nil {
		return "", err
	}
	return joinPath(base, parts...), nil
}

// New ND path builders

// ndPath builds a path for the new ND API root
// Example: /api/v1/inventory, /api/v1/telemetry
func (c *Client) ndPath(parts ...string) (string, error) {
	base, err := c.endpoints.base(APINDRootV1)
	if err != nil {
		return "", err
	}
	return joinPath(base, parts...), nil
}

// ndLanFabricPath builds a path for the new ND LAN fabric API
// Example: /api/v1/lan-fabric/fabrics/{fabricID}/switches
func (c *Client) ndLanFabricPath(parts ...string) (string, error) {
	return c.ndPath(append([]string{"lan-fabric"}, parts...)...)
}

// addQuery appends query parameters to a path
func addQuery(path string, vals url.Values) string {
	if len(vals) == 0 {
		return path
	}
	return path + "?" + vals.Encode()
}
