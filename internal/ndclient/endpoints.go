package ndclient

import (
	"fmt"
	"net/url"
	"strings"
)

// APINamespace identifies a specific API surface (legacy NDFC, new ND, etc.)
// Names encode version for future compatibility (e.g., ndfc.security.v1)
type APINamespace string

const (
	// Legacy NDFC APIs (under /appcenter/cisco/ndfc/api/v1/)
	APINDFCSecurityV1        APINamespace = "ndfc.security.v1"
	APINDFCLANFabricV1       APINamespace = "ndfc.lan-fabric.v1"
	APINDFCImageManagementV1 APINamespace = "ndfc.imagemanagement.v1"

	// New ND APIs (under /api/v1/)
	APINDRootV1   APINamespace = "nd.root.v1"   // Base for all new ND APIs
	APINDManageV1 APINamespace = "nd.manage.v1" // /api/v1/manage
)

// Endpoints holds configurable base paths for each API namespace
type Endpoints struct {
	Base map[APINamespace]string
}

// DefaultEndpoints returns the standard endpoint configuration
func DefaultEndpoints() Endpoints {
	return Endpoints{
		Base: map[APINamespace]string{
			// Legacy NDFC APIs
			APINDFCSecurityV1:        "/appcenter/cisco/ndfc/api/v1/security",
			APINDFCLANFabricV1:       "/appcenter/cisco/ndfc/api/v1/lan-fabric",
			APINDFCImageManagementV1: "/appcenter/cisco/ndfc/api/v1/imagemanagement",

			// New ND APIs
			APINDRootV1:   "/api/v1",
			APINDManageV1: "/api/v1/manage",
		},
	}
}

// base returns the base path for a namespace, or error if not configured
// Trims whitespace and trailing slashes for consistent path building
func (e Endpoints) base(ns APINamespace) (string, error) {
	b, ok := e.Base[ns]
	b = strings.TrimRight(strings.TrimSpace(b), "/")
	if !ok || b == "" {
		return "", fmt.Errorf("missing base path for namespace %q", ns)
	}
	return b, nil
}

// joinPath builds a URL path from a base and escaped path segments
func joinPath(base string, parts ...string) string {
	p := strings.TrimRight(base, "/")
	for _, part := range parts {
		p += "/" + url.PathEscape(part)
	}
	return p
}
