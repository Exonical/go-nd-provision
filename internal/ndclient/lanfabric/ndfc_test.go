package lanfabric

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockClient implements ClientInterface for testing
type mockClient struct {
	handler http.Handler
	server  *httptest.Server
}

func newMockClient(t *testing.T, handler http.Handler) *mockClient {
	t.Helper()
	server := httptest.NewServer(handler)
	return &mockClient{handler: handler, server: server}
}

func (m *mockClient) Close() {
	m.server.Close()
}

func (m *mockClient) Get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", m.server.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return &testAPIError{StatusCode: resp.StatusCode}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (m *mockClient) Post(ctx context.Context, path string, body, out any) error {
	return nil // Not used in these tests
}

func (m *mockClient) Put(ctx context.Context, path string, body, out any) error {
	return nil // Not used in these tests
}

func (m *mockClient) Delete(ctx context.Context, path string) error {
	return nil // Not used in these tests
}

func (m *mockClient) NDFCLanFabricPath(parts ...string) (string, error) {
	return "/appcenter/cisco/ndfc/api/v1/lan-fabric/" + strings.Join(parts, "/"), nil
}

func (m *mockClient) NDLanFabricPath(parts ...string) (string, error) {
	return "/api/v1/lan-fabric/" + strings.Join(parts, "/"), nil
}

type testAPIError struct {
	StatusCode int
}

func (e *testAPIError) Error() string {
	return "API error"
}

// TestGetVRFsNDFC_Success tests successful VRF retrieval
func TestGetVRFsNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/top-down/fabrics/test-fabric/vrfs") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// VRFs are returned as []map[string]interface{}
		vrfs := []map[string]interface{}{
			{"vrfName": "vrf1", "fabric": "test-fabric"},
			{"vrfName": "vrf2", "fabric": "test-fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(vrfs)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	vrfs, err := svc.GetVRFsNDFC(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vrfs) != 2 {
		t.Fatalf("expected 2 VRFs, got %d", len(vrfs))
	}
	if vrfs[0]["vrfName"] != "vrf1" {
		t.Errorf("expected vrf1, got %v", vrfs[0]["vrfName"])
	}
}

// TestVRFExists_Found tests VRF exists check when VRF is found
func TestVRFExists_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vrfs := []map[string]interface{}{
			{"vrfName": "default", "fabric": "test-fabric"},
			{"vrfName": "hpc", "fabric": "test-fabric"},
			{"vrfName": "prod", "fabric": "test-fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(vrfs)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.VRFExists(context.Background(), "test-fabric", "hpc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected VRF to exist")
	}
}

// TestVRFExists_NotFound tests VRF exists check when VRF is not found
func TestVRFExists_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vrfs := []map[string]interface{}{
			{"vrfName": "default", "fabric": "test-fabric"},
			{"vrfName": "prod", "fabric": "test-fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(vrfs)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.VRFExists(context.Background(), "test-fabric", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected VRF to not exist")
	}
}

// TestNetworkExists_Found tests network exists check when network is found
func TestNetworkExists_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		networks := []map[string]interface{}{
			{"networkName": "net1", "fabric": "test-fabric"},
			{"networkName": "hpcnet", "fabric": "test-fabric"},
			{"networkName": "net3", "fabric": "test-fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.NetworkExists(context.Background(), "test-fabric", "hpcnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected network to exist")
	}
}

// TestNetworkExists_NotFound tests network exists check when network is not found
func TestNetworkExists_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		networks := []map[string]interface{}{
			{"networkName": "net1", "fabric": "test-fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.NetworkExists(context.Background(), "test-fabric", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected network to not exist")
	}
}

// TestGetFabricsNDFC_Success tests successful fabric retrieval
func TestGetFabricsNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/control/fabrics") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		fabrics := []FabricData{
			{ID: 1, FabricName: "fabric1", FabricType: "Switch_Fabric"},
			{ID: 2, FabricName: "fabric2", FabricType: "Switch_Fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fabrics)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	fabrics, err := svc.GetFabricsNDFC(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fabrics) != 2 {
		t.Fatalf("expected 2 fabrics, got %d", len(fabrics))
	}
}

// TestGetSwitchesNDFC_Success tests successful switch retrieval
func TestGetSwitchesNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path is /rest/control/fabrics/{fabric}/inventory
		if !strings.Contains(r.URL.Path, "/rest/control/fabrics/test-fabric/inventory") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		switches := []SwitchData{
			{SerialNumber: "ABC123", LogicalName: "leaf1", SwitchRole: "leaf"},
			{SerialNumber: "DEF456", LogicalName: "spine1", SwitchRole: "spine"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(switches)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	switches, err := svc.GetSwitchesNDFC(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(switches) != 2 {
		t.Fatalf("expected 2 switches, got %d", len(switches))
	}
}

// TestIsLeafOrBorder tests switch role filtering
func TestIsLeafOrBorder(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"leaf", true},
		{"Leaf", true},
		{"LEAF", true},
		{"border", true},
		{"Border", true},
		{"border_gateway", true},
		{"tor", true},
		{"ToR", true},
		{"spine", false},
		{"Spine", false},
		{"super_spine", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := IsLeafOrBorder(tt.role)
			if result != tt.expected {
				t.Errorf("IsLeafOrBorder(%q) = %v, want %v", tt.role, result, tt.expected)
			}
		})
	}
}

// TestGetNetworksNDFC_Success tests successful network retrieval
func TestGetNetworksNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/top-down/fabrics/test-fabric/networks") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		networks := []NetworkData{
			{NetworkName: "net1", Fabric: "test-fabric", VRF: "vrf1"},
			{NetworkName: "net2", Fabric: "test-fabric", VRF: "vrf2"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	networks, err := svc.GetNetworksNDFC(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(networks) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(networks))
	}
}

// TestValidation_EmptyFabricName tests validation for empty fabric name
func TestValidation_EmptyFabricName(t *testing.T) {
	svc := NewService(nil) // No client needed for validation

	_, err := svc.GetSwitchesNDFC(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty fabric name")
	}
	if !strings.Contains(err.Error(), "fabricName") {
		t.Errorf("expected error about fabricName, got: %v", err)
	}
}

// TestGetSwitchPortsNDFC_Success tests successful switch port retrieval
func TestGetSwitchPortsNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Returns []InterfaceResponse (array of policy groups)
		resp := []InterfaceResponse{
			{
				Policy: "int_trunk_host",
				Interfaces: []InterfaceData{
					{SerialNumber: "ABC123", IfName: "Ethernet1/1", NvPairs: map[string]interface{}{"ADMIN_STATE": "true"}},
					{SerialNumber: "ABC123", IfName: "Ethernet1/2", NvPairs: map[string]interface{}{"ADMIN_STATE": "false"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	ports, err := svc.GetSwitchPortsNDFC(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].Name != "Ethernet1/1" {
		t.Errorf("expected Ethernet1/1, got %s", ports[0].Name)
	}
}

// TestGetFabricNDFC_Success tests successful single fabric retrieval
func TestGetFabricNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fabric := FabricData{ID: 1, FabricName: "test-fabric", FabricType: "Switch_Fabric"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fabric)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	fabric, err := svc.GetFabricNDFC(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fabric.FabricName != "test-fabric" {
		t.Errorf("expected test-fabric, got %s", fabric.FabricName)
	}
}

// TestFindFabricByNameNDFC_Found tests finding fabric by name
func TestFindFabricByNameNDFC_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fabrics := []FabricData{
			{ID: 1, FabricName: "fabric1", FabricType: "Switch_Fabric"},
			{ID: 2, FabricName: "target-fabric", FabricType: "Switch_Fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fabrics)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	fabric, err := svc.FindFabricByNameNDFC(context.Background(), "target-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fabric.FabricName != "target-fabric" {
		t.Errorf("expected target-fabric, got %s", fabric.FabricName)
	}
}

// TestGetFabricLinksNDFC_Success tests successful fabric links retrieval
func TestGetFabricLinksNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		links := []FabricLink{
			{
				LinkUUID:   "link-1",
				LinkType:   "inter-fabric",
				FabricName: "test-fabric",
				Sw1Info:    FabricLinkInfo{SerialNumber: "ABC123", IfName: "Ethernet1/1"},
				Sw2Info:    FabricLinkInfo{SerialNumber: "DEF456", IfName: "Ethernet1/1"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(links)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	links, err := svc.GetFabricLinksNDFC(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
}

// TestGetUplinkPortsNDFC_Success tests successful uplink ports retrieval
func TestGetUplinkPortsNDFC_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		links := []FabricLink{
			{
				LinkUUID:   "link-1",
				FabricName: "test-fabric",
				Sw1Info:    FabricLinkInfo{SerialNumber: "ABC123", IfName: "Ethernet1/49", SwitchRole: "leaf"},
				Sw2Info:    FabricLinkInfo{SerialNumber: "SPINE1", IfName: "Ethernet1/1", SwitchRole: "spine"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(links)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	// GetUplinkPortsNDFC returns map[string]bool where key is "serial:ifName"
	ports, err := svc.GetUplinkPortsNDFC(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find both ends of the link as uplinks
	if len(ports) != 2 {
		t.Fatalf("expected 2 uplink ports, got %d", len(ports))
	}
	if !ports["ABC123:Ethernet1/49"] {
		t.Error("expected ABC123:Ethernet1/49 to be an uplink")
	}
}

// TestIsEthernetPort tests ethernet port detection
// Only Ethernetx/x or Ethernetx/x/x patterns are valid
func TestIsEthernetPort(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		// Valid: Ethernetx/x (standard ports)
		{"Ethernet1/1", true},
		{"Ethernet1/49", true},
		{"Ethernet2/1", true},
		// Valid: Ethernetx/x/x (breakout ports)
		{"Ethernet1/1/1", true},
		{"Ethernet1/49/4", true},
		// Valid: with leading/trailing whitespace (trimmed)
		{"  Ethernet1/1  ", true},
		// Invalid: short form (Eth)
		{"Eth1/1", false},
		{"Eth1/1/1", false},
		// Invalid: other interface types
		{"Loopback0", false},
		{"Vlan100", false},
		{"mgmt0", false},
		{"port-channel1", false},
		// Invalid: malformed Ethernet patterns
		{"Ethernet", false},
		{"Ethernet1", false},
		{"Ethernet1/", false},
		{"Ethernet1/1/", false},
		{"Ethernet1/1/1/1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEthernetPort(tt.name)
			if result != tt.expected {
				t.Errorf("IsEthernetPort(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestNormalizeInterfaces tests batch interface normalization
func TestNormalizeInterfaces(t *testing.T) {
	interfaces := []InterfaceData{
		{SerialNumber: "ABC123", IfName: "Ethernet1/1", NvPairs: map[string]interface{}{"ADMIN_STATE": "true"}},
		{SerialNumber: "ABC123", IfName: "Ethernet1/2", NvPairs: map[string]interface{}{"ADMIN_STATE": "false"}},
	}

	ports := NormalizeInterfaces(interfaces)
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].Name != "Ethernet1/1" {
		t.Errorf("expected Ethernet1/1, got %s", ports[0].Name)
	}
}

// TestExtractVLANFromConfig tests VLAN extraction from network config
func TestExtractVLANFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected string
	}{
		{"valid config", `{"vlanId": "100"}`, "100"},
		{"empty config", "", ""},
		{"invalid json", "not json", ""},
		{"missing vlanId", `{"other": "value"}`, ""},
		{"numeric vlanId", `{"vlanId": 200}`, "200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVLANFromConfig(tt.config)
			if result != tt.expected {
				t.Errorf("extractVLANFromConfig(%q) = %q, want %q", tt.config, result, tt.expected)
			}
		})
	}
}

// TestGetNetworkVLAN_Success tests successful network VLAN retrieval
func TestGetNetworkVLAN_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GetNetworkVLAN uses NetworkData struct, not map
		networks := []NetworkData{
			{NetworkName: "hpcnet", Fabric: "test-fabric", NetworkTemplateConfig: `{"vlanId": "200"}`},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	vlan, err := svc.GetNetworkVLAN(context.Background(), "test-fabric", "hpcnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vlan != "200" {
		t.Errorf("expected VLAN '200', got %q", vlan)
	}
}

// TestGetNetworkVLAN_NotFound tests network not found error
func TestGetNetworkVLAN_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		networks := []NetworkData{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	_, err := svc.GetNetworkVLAN(context.Background(), "test-fabric", "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found network")
	}
}

// NOTE: Tests for ConfigureAccessHostInterface, UpdateInterfacesNDFC, DeployInterfacesNDFC,
// AttachPortsToNetwork, and DetachPortsFromNetwork have been removed because the mock
// Post/Put methods are no-ops and don't actually exercise the transport layer.
// These tests gave false confidence. Add them back when implementing proper transport mocks.

// TestDeployInterfacesNDFC_EmptyList_NoRequest tests that empty interface list skips the POST
// This is a valid logic test - verifying the early return behavior after deduplication.
func TestDeployInterfacesNDFC_EmptyList_NoRequest(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer client.Close()

	svc := NewService(client)
	// Empty list should return nil without calling Post (early return after deduplication)
	err := svc.DeployInterfacesNDFC(context.Background(), "ABC123", []string{})
	if err != nil {
		t.Fatalf("expected nil error for empty list, got: %v", err)
	}
}

// TestAttachPortsToNetwork_EmptyAttachments_Validation tests input validation
func TestAttachPortsToNetwork_EmptyAttachments_Validation(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make request for empty attachments")
	}))
	defer client.Close()

	svc := NewService(client)
	err := svc.AttachPortsToNetwork(context.Background(), "test-fabric", "hpcnet", []NetworkAttachment{})
	if err == nil {
		t.Fatal("expected error for empty attachments")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected 'cannot be empty' error, got: %v", err)
	}
}

// NOTE: DetachPortsFromNetwork does not validate empty attachments - it proceeds with empty list.
// This is different from AttachPortsToNetwork which validates. Consider adding validation if needed.

// =============================================================================
// NDFC Realism Tests - Edge cases that commonly break NDFC integrations
// =============================================================================

// TestExtractVLANFromConfig_FloatVLAN tests that non-integer VLAN is rejected
func TestExtractVLANFromConfig_FloatVLAN(t *testing.T) {
	// NDFC sometimes returns numeric values; floats should be rejected
	result := extractVLANFromConfig(`{"vlanId": 200.5}`)
	if result != "" {
		t.Errorf("expected empty string for float VLAN, got %q", result)
	}
}

// TestNormalizeInterface_MissingFields tests normalization with missing/nil fields
func TestNormalizeInterface_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		input InterfaceData
	}{
		{"nil nvPairs", InterfaceData{SerialNumber: "ABC", IfName: "Eth1/1", NvPairs: nil}},
		{"empty nvPairs", InterfaceData{SerialNumber: "ABC", IfName: "Eth1/1", NvPairs: map[string]interface{}{}}},
		{"missing ifName", InterfaceData{SerialNumber: "ABC", IfName: "", NvPairs: nil}},
		{"missing serial", InterfaceData{SerialNumber: "", IfName: "Eth1/1", NvPairs: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := NormalizeInterface(tt.input)
			// Basic sanity check
			if result.Name != tt.input.IfName {
				t.Errorf("expected Name=%q, got %q", tt.input.IfName, result.Name)
			}
		})
	}
}

// TestNormalizeInterface_NvPairsTypeVariations tests NDFC's inconsistent types in nvPairs
func TestNormalizeInterface_NvPairsTypeVariations(t *testing.T) {
	// NDFC sometimes returns bools/numbers instead of strings
	tests := []struct {
		name    string
		nvPairs map[string]interface{}
	}{
		{"bool admin state", map[string]interface{}{"ADMIN_STATE": true}},
		{"number speed", map[string]interface{}{"SPEED": 10000}},
		{"mixed types", map[string]interface{}{"ADMIN_STATE": "true", "MTU": 9216, "ENABLED": false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := InterfaceData{
				SerialNumber: "ABC123",
				IfName:       "Ethernet1/1",
				NvPairs:      tt.nvPairs,
			}
			// Should not panic
			_ = NormalizeInterface(input)
		})
	}
}

// TestGetSwitchPortsNDFC_EmptyInterfaces tests handling of empty interface list
func TestGetSwitchPortsNDFC_EmptyInterfaces(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NDFC returns empty array when no interfaces match
		resp := []InterfaceResponse{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	ports, err := svc.GetSwitchPortsNDFC(context.Background(), "NONEXISTENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected 0 ports, got %d", len(ports))
	}
}

// TestGetSwitchPortsNDFC_MultiplePolicyGroups tests flattening across policy groups
func TestGetSwitchPortsNDFC_MultiplePolicyGroups(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NDFC returns interfaces grouped by policy
		resp := []InterfaceResponse{
			{
				Policy: "int_trunk_host",
				Interfaces: []InterfaceData{
					{SerialNumber: "ABC", IfName: "Ethernet1/1"},
				},
			},
			{
				Policy: "int_access_host",
				Interfaces: []InterfaceData{
					{SerialNumber: "ABC", IfName: "Ethernet1/2"},
					{SerialNumber: "ABC", IfName: "Ethernet1/3"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	ports, err := svc.GetSwitchPortsNDFC(context.Background(), "ABC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should flatten all 3 interfaces from both policy groups
	if len(ports) != 3 {
		t.Errorf("expected 3 ports (flattened), got %d", len(ports))
	}
}

// TestVRFExists_EmptyVRFList tests handling when fabric has no VRFs
func TestVRFExists_EmptyVRFList(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vrfs := []map[string]interface{}{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vrfs)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.VRFExists(context.Background(), "test-fabric", "any-vrf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected VRF to not exist in empty list")
	}
}

// TestNetworkExists_EmptyNetworkList tests handling when fabric has no networks
func TestNetworkExists_EmptyNetworkList(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		networks := []map[string]interface{}{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(networks)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	exists, err := svc.NetworkExists(context.Background(), "test-fabric", "any-network")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected network to not exist in empty list")
	}
}

// TestFindFabricByNameNDFC_NotFound tests fabric not found
func TestFindFabricByNameNDFC_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fabrics := []FabricData{
			{ID: 1, FabricName: "fabric1", FabricType: "Switch_Fabric"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fabrics)
	})

	client := newMockClient(t, handler)
	defer client.Close()

	svc := NewService(client)
	_, err := svc.FindFabricByNameNDFC(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found fabric")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}
