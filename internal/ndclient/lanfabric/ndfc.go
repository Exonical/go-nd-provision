package lanfabric

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/banglin/go-nd/internal/ndclient/common"
)

// Service provides LAN fabric operations
type Service struct {
	client ClientInterface
}

// ClientInterface defines the methods needed from the main client
type ClientInterface interface {
	Get(ctx context.Context, path string, result interface{}) error
	Put(ctx context.Context, path string, body, result interface{}) error
	Post(ctx context.Context, path string, body, result interface{}) error
	NDFCLanFabricPath(parts ...string) (string, error)
	NDLanFabricPath(parts ...string) (string, error)
}

// NewService creates a new LAN fabric service
func NewService(client ClientInterface) *Service {
	return &Service{client: client}
}

// GetFabricsNDFC retrieves all fabrics from legacy NDFC API
func (s *Service) GetFabricsNDFC(ctx context.Context) ([]FabricData, error) {
	path, err := s.client.NDFCLanFabricPath("fabrics")
	if err != nil {
		return nil, err
	}

	var response FabricResponse
	if err := s.client.Get(ctx, path, &response); err != nil {
		return nil, fmt.Errorf("get fabrics (ndfc): %w", err)
	}
	return response.Fabrics, nil
}

// GetFabricNDFC retrieves a single fabric by ID from legacy NDFC API
func (s *Service) GetFabricNDFC(ctx context.Context, fabricID string) (*FabricData, error) {
	if err := common.RequireNonEmpty("fabricID", fabricID); err != nil {
		return nil, err
	}

	path, err := s.client.NDFCLanFabricPath("fabrics", fabricID)
	if err != nil {
		return nil, err
	}

	var fabric FabricData
	if err := s.client.Get(ctx, path, &fabric); err != nil {
		return nil, fmt.Errorf("get fabric (ndfc, fabricID=%s): %w", fabricID, err)
	}
	return &fabric, nil
}

// FindFabricByNameNDFC searches for a fabric by name using legacy NDFC API
func (s *Service) FindFabricByNameNDFC(ctx context.Context, name string) (*FabricData, error) {
	if err := common.RequireNonEmpty("name", name); err != nil {
		return nil, err
	}

	fabrics, err := s.GetFabricsNDFC(ctx)
	if err != nil {
		return nil, err
	}
	for i := range fabrics {
		if fabrics[i].Name == name {
			return &fabrics[i], nil
		}
	}
	return nil, fmt.Errorf("fabric not found (ndfc): %q", name)
}

// GetFabricLinksNDFC retrieves all inter-switch links for a fabric from NDFC
// Uses: /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/control/links/fabrics/{fabricName}
func (s *Service) GetFabricLinksNDFC(ctx context.Context, fabricName string) ([]FabricLink, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := s.client.NDFCLanFabricPath("rest", "control", "links", "fabrics", fabricName)
	if err != nil {
		return nil, err
	}

	var links []FabricLink
	if err := s.client.Get(ctx, path, &links); err != nil {
		return nil, fmt.Errorf("get fabric links (ndfc, fabric=%s): %w", fabricName, err)
	}
	return links, nil
}

// GetUplinkPortsNDFC returns a set of serial:ifName for all ports used in inter-switch links
func (s *Service) GetUplinkPortsNDFC(ctx context.Context, fabricName string) (map[string]bool, error) {
	links, err := s.GetFabricLinksNDFC(ctx, fabricName)
	if err != nil {
		return nil, err
	}

	uplinks := make(map[string]bool)
	for _, link := range links {
		if link.Sw1Info.SerialNumber != "" && link.Sw1Info.IfName != "" {
			key := link.Sw1Info.SerialNumber + ":" + link.Sw1Info.IfName
			uplinks[key] = true
		}
		if link.Sw2Info.SerialNumber != "" && link.Sw2Info.IfName != "" {
			key := link.Sw2Info.SerialNumber + ":" + link.Sw2Info.IfName
			uplinks[key] = true
		}
	}
	return uplinks, nil
}

// GetSwitchesNDFC retrieves all switches for a fabric from legacy NDFC API
// Uses: /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/control/fabrics/{fabricName}/inventory
func (s *Service) GetSwitchesNDFC(ctx context.Context, fabricName string) ([]SwitchData, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}

	path, err := s.client.NDFCLanFabricPath("rest", "control", "fabrics", fabricName, "inventory")
	if err != nil {
		return nil, err
	}

	var switches []SwitchData
	if err := s.client.Get(ctx, path, &switches); err != nil {
		return nil, fmt.Errorf("get switches (ndfc, fabric=%s): %w", fabricName, err)
	}
	return switches, nil
}

// GetSwitchPortsNDFC retrieves all interfaces for a switch from legacy NDFC API
// Uses: /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface?serialNumber=XXX
// Returns normalized SwitchPortData
func (s *Service) GetSwitchPortsNDFC(ctx context.Context, serialNumber string) ([]SwitchPortData, error) {
	if err := common.RequireNonEmpty("serialNumber", serialNumber); err != nil {
		return nil, err
	}

	path, err := s.client.NDFCLanFabricPath("rest", "interface")
	if err != nil {
		return nil, err
	}
	path = addQuery(path, url.Values{"serialNumber": {serialNumber}})

	var responses []InterfaceResponse
	if err := s.client.Get(ctx, path, &responses); err != nil {
		return nil, fmt.Errorf("get interfaces (ndfc, serial=%s): %w", serialNumber, err)
	}

	// Flatten all interfaces from all policy groups and normalize
	var ports []SwitchPortData
	for _, resp := range responses {
		for _, iface := range resp.Interfaces {
			ports = append(ports, NormalizeInterface(iface))
		}
	}
	return ports, nil
}

// IsLeafOrBorder returns true if the switch role is leaf, tor, or border (not spine)
func IsLeafOrBorder(role string) bool {
	r := strings.ToLower(role)
	if strings.Contains(r, "spine") {
		return false
	}
	return strings.Contains(r, "leaf") || strings.Contains(r, "tor") || strings.Contains(r, "border")
}

// IsEthernetPort returns true if the port name is an Ethernet interface
// Handles both "Ethernet1/1" and "Eth1/1" formats
func IsEthernetPort(name string) bool {
	return strings.HasPrefix(name, "Ethernet") || strings.HasPrefix(name, "Eth")
}

// addQuery appends query parameters to a path
func addQuery(path string, vals url.Values) string {
	if len(vals) == 0 {
		return path
	}
	return path + "?" + vals.Encode()
}

// GetNetworksNDFC returns all networks for a fabric
func (s *Service) GetNetworksNDFC(ctx context.Context, fabricName string) ([]map[string]interface{}, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return nil, err
	}
	path, err := s.client.NDFCLanFabricPath("rest", "top-down", "fabrics", fabricName, "networks")
	if err != nil {
		return nil, err
	}
	var networks []map[string]interface{}
	if err := s.client.Get(ctx, path, &networks); err != nil {
		return nil, fmt.Errorf("get networks (ndfc, fabric=%s): %w", fabricName, err)
	}
	return networks, nil
}

// UpdateInterfacesNDFC updates interface configurations in NDFC
// PUT /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface
func (s *Service) UpdateInterfacesNDFC(ctx context.Context, req *InterfaceUpdateRequest) error {
	path, err := s.client.NDFCLanFabricPath("rest", "interface")
	if err != nil {
		return err
	}
	var result interface{}
	if err := s.client.Put(ctx, path, req, &result); err != nil {
		// Try to extract body from APIError for better debugging
		if apiErr, ok := err.(interface{ BodyString(int) string }); ok {
			return fmt.Errorf("update interface (policy=%s): %w, body: %s", req.Policy, err, apiErr.BodyString(500))
		}
		return fmt.Errorf("update interface (policy=%s): %w", req.Policy, err)
	}
	return nil
}

// DeployInterfacesNDFC deploys interface configurations to devices
// POST /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface/deploy
func (s *Service) DeployInterfacesNDFC(ctx context.Context, serialNumber string, ifNames []string) error {
	path, err := s.client.NDFCLanFabricPath("rest", "interface", "deploy")
	if err != nil {
		return err
	}

	// Dedupe interface names to avoid duplicate deploy requests
	seen := make(map[string]bool)
	var req InterfaceDeployRequest
	for _, ifName := range ifNames {
		if seen[ifName] {
			continue
		}
		seen[ifName] = true
		req = append(req, InterfaceDeployItem{
			SerialNumber: serialNumber,
			IfName:       ifName,
		})
	}

	if len(req) == 0 {
		return nil
	}

	var result interface{}
	return s.client.Post(ctx, path, req, &result)
}

// ConfigureAccessHostInterface configures an interface with int_access_host policy
// This sets up access mode with VLAN, PFC, QoS, and other interface settings
func (s *Service) ConfigureAccessHostInterface(ctx context.Context, serialNumber, ifName, accessVlan, description string) error {
	nvPairs := map[string]interface{}{
		"ADMIN_STATE":           "true",
		"SPEED":                 "Auto",
		"MTU":                   "jumbo",
		"DESC":                  description,
		"ACCESS_VLAN":           accessVlan,
		"BPDUGUARD_ENABLED":     "true",
		"PORTTYPE_FAST_ENABLED": "true",
		"ENABLE_NETFLOW":        "false",
		"ENABLE_PFC":            "true",
		"ENABLE_QOS":            "true",
		"CONF":                  "",
	}

	req := &InterfaceUpdateRequest{
		Policy: "int_access_host",
		Interfaces: []InterfaceUpdateConfig{
			{
				SerialNumber: serialNumber,
				IfName:       ifName,
				NvPairs:      nvPairs,
			},
		},
	}
	return s.UpdateInterfacesNDFC(ctx, req)
}

// GetNetworkVLAN retrieves the VLAN ID for a network from NDFC
// GET /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/top-down/fabrics/{fabricName}/networks
func (s *Service) GetNetworkVLAN(ctx context.Context, fabricName, networkName string) (string, error) {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return "", err
	}
	if err := common.RequireNonEmpty("networkName", networkName); err != nil {
		return "", err
	}

	path, err := s.client.NDFCLanFabricPath("rest", "top-down", "fabrics", fabricName, "networks")
	if err != nil {
		return "", err
	}

	var networks []NetworkData
	if err := s.client.Get(ctx, path, &networks); err != nil {
		return "", fmt.Errorf("failed to get networks: %w", err)
	}

	// Find the network by name
	for _, net := range networks {
		if net.NetworkName == networkName {
			// Parse the networkTemplateConfig JSON to extract vlanId
			// The config is a JSON string like: {"vlanId":"2301",...}
			vlanID := extractVLANFromConfig(net.NetworkTemplateConfig)
			if vlanID == "" {
				return "", fmt.Errorf("network %s has no VLAN configured", networkName)
			}
			return vlanID, nil
		}
	}

	return "", fmt.Errorf("network %s not found in fabric %s", networkName, fabricName)
}

// extractVLANFromConfig extracts the vlanId from the networkTemplateConfig JSON string
func extractVLANFromConfig(config string) string {
	// Try JSON unmarshal first (handles numeric values, spacing variations, etc.)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(config), &m); err == nil {
		if v, ok := m["vlanId"]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				if t != float64(int(t)) {
					return "" // Non-integer VLAN is invalid
				}
				return strconv.Itoa(int(t))
			}
		}
	}

	// Fallback: simple string parsing for edge cases
	const prefix = `"vlanId":"`
	idx := strings.Index(config, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(config[start:], `"`)
	if end == -1 {
		return ""
	}
	return config[start : start+end]
}

// AttachPortsToNetwork attaches switch ports to a network by name
// This automatically configures the correct VLAN based on the network definition
// POST /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/top-down/fabrics/{fabricName}/networks/attachments
func (s *Service) AttachPortsToNetwork(ctx context.Context, fabricName, networkName string, attachments []NetworkAttachment) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("networkName", networkName); err != nil {
		return err
	}
	if len(attachments) == 0 {
		return fmt.Errorf("attachments cannot be empty")
	}

	path, err := s.client.NDFCLanFabricPath("rest", "top-down", "fabrics", fabricName, "networks", "attachments")
	if err != nil {
		return err
	}

	// Build the request - wrap attachments in the network request structure
	req := []NetworkAttachRequest{
		{
			NetworkName:   networkName,
			LanAttachList: attachments,
		},
	}

	// Decode into concrete type - NDFC returns map of network->status
	var result map[string]string
	if err := s.client.Post(ctx, path, req, &result); err != nil {
		if apiErr, ok := err.(interface{ BodyString(int) string }); ok {
			return fmt.Errorf("attach ports to network (fabric=%s, network=%s): %w, body: %s", fabricName, networkName, err, apiErr.BodyString(500))
		}
		return fmt.Errorf("attach ports to network (fabric=%s, network=%s): %w", fabricName, networkName, err)
	}

	// Check for non-SUCCESS statuses in response
	for key, status := range result {
		if strings.ToUpper(status) != "SUCCESS" {
			return fmt.Errorf("attach ports to network failed for %s: %s", key, status)
		}
	}

	return nil
}

// DetachPortsFromNetwork detaches switch ports from a network
func (s *Service) DetachPortsFromNetwork(ctx context.Context, fabricName, networkName string, attachments []NetworkAttachment) error {
	if err := common.RequireNonEmpty("fabricName", fabricName); err != nil {
		return err
	}
	if err := common.RequireNonEmpty("networkName", networkName); err != nil {
		return err
	}

	path, err := s.client.NDFCLanFabricPath("rest", "top-down", "fabrics", fabricName, "networks", "attachments")
	if err != nil {
		return err
	}

	// For detach, set the ports in detachSwitchPorts and clear switchPorts
	for i := range attachments {
		attachments[i].DetachSwitchPorts = attachments[i].SwitchPorts
		attachments[i].SwitchPorts = ""
		attachments[i].Deployment = true
	}

	req := []NetworkAttachRequest{
		{
			NetworkName:   networkName,
			LanAttachList: attachments,
		},
	}

	var result map[string]string
	if err := s.client.Post(ctx, path, req, &result); err != nil {
		if apiErr, ok := err.(interface{ BodyString(int) string }); ok {
			return fmt.Errorf("detach ports from network (fabric=%s, network=%s): %w, body: %s", fabricName, networkName, err, apiErr.BodyString(500))
		}
		return fmt.Errorf("detach ports from network (fabric=%s, network=%s): %w", fabricName, networkName, err)
	}

	// Check for non-SUCCESS statuses in response
	for key, status := range result {
		if strings.ToUpper(status) != "SUCCESS" {
			return fmt.Errorf("detach ports from network failed for %s: %s", key, status)
		}
	}

	return nil
}
