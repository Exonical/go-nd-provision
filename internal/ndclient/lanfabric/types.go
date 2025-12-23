package lanfabric

// FabricResponse wraps the fabric list response
type FabricResponse struct {
	Fabrics []FabricData `json:"fabrics"`
}

// FabricData represents a fabric from NDFC
type FabricData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SwitchData represents a switch from the NDFC inventory endpoint
type SwitchData struct {
	SwitchDbID   int    `json:"switchDbID"`
	SerialNumber string `json:"serialNumber"`
	LogicalName  string `json:"logicalName"`
	Model        string `json:"model"`
	IPAddress    string `json:"ipAddress"`
	FabricName   string `json:"fabricName"`
	SwitchRole   string `json:"switchRoleEnum"`
	Release      string `json:"release"`
	Status       string `json:"status"`
	Health       int    `json:"health"`
}

// InterfaceResponse is the response from the NDFC interface API
// GET /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface?serialNumber=XXX
type InterfaceResponse struct {
	Policy     string          `json:"policy"`
	Interfaces []InterfaceData `json:"interfaces"`
}

// InterfaceData represents a single interface from NDFC (raw API response)
type InterfaceData struct {
	SerialNumber string                 `json:"serialNumber"`
	IfName       string                 `json:"ifName"`
	NvPairs      map[string]interface{} `json:"nvPairs"`
}

// SwitchPortData is the normalized port data for our database
type SwitchPortData struct {
	SerialNumber string
	Name         string
	Description  string
	AdminState   string
	Speed        string
	MTU          string
}

// FabricLink represents a link between switches from NDFC
type FabricLink struct {
	LinkUUID   string         `json:"link-uuid"`
	LinkType   string         `json:"link-type"`
	FabricName string         `json:"fabricName"`
	Sw1Info    FabricLinkInfo `json:"sw1-info"`
	Sw2Info    FabricLinkInfo `json:"sw2-info"`
}

// FabricLinkInfo contains switch/interface info for one end of a link
type FabricLinkInfo struct {
	SerialNumber string `json:"sw-serial-number"`
	IfName       string `json:"if-name"`
	SwitchRole   string `json:"switch-role"`
	SysName      string `json:"sw-sys-name"`
}

// InterfaceUpdateRequest is the payload for updating interfaces via NDFC API
// PUT /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface
type InterfaceUpdateRequest struct {
	Policy     string                  `json:"policy"`
	Interfaces []InterfaceUpdateConfig `json:"interfaces"`
}

// InterfaceUpdateConfig represents a single interface configuration for update
type InterfaceUpdateConfig struct {
	SerialNumber string                 `json:"serialNumber"`
	IfName       string                 `json:"ifName"`
	NvPairs      map[string]interface{} `json:"nvPairs"` // Object of nvPairs
}

// InterfaceDeployItem is a single interface to deploy
type InterfaceDeployItem struct {
	SerialNumber string `json:"serialNumber"`
	IfName       string `json:"ifName"`
}

// InterfaceDeployRequest is the payload for deploying interface configs
// POST /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/interface/deploy
// The API expects an array of InterfaceDeployItem
type InterfaceDeployRequest []InterfaceDeployItem

// NetworkData represents a network from NDFC
// GET /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/top-down/fabrics/{fabricName}/networks
type NetworkData struct {
	ID                    int    `json:"id"`
	Fabric                string `json:"fabric"`
	NetworkName           string `json:"networkName"`
	NetworkID             int    `json:"networkId"`
	VRF                   string `json:"vrf"`
	NetworkTemplateConfig string `json:"networkTemplateConfig"` // JSON string containing vlanId
}

// NetworkAttachRequest is the payload for attaching ports to a network
// POST /appcenter/cisco/ndfc/api/v1/lan-fabric/rest/top-down/fabrics/{fabricName}/networks/attachments
type NetworkAttachRequest struct {
	NetworkName   string              `json:"networkName"`
	LanAttachList []NetworkAttachment `json:"lanAttachList"`
}

// NetworkAttachment represents a single port attachment to a network
// All fields are required by NDFC API - empty strings must be sent, not omitted
type NetworkAttachment struct {
	Deployment        bool    `json:"deployment"`
	DetachSwitchPorts string  `json:"detachSwitchPorts"`
	Dot1QVlan         int     `json:"dot1QVlan"`
	ExtensionValues   string  `json:"extensionValues"`
	Fabric            string  `json:"fabric"`
	FreeformConfig    string  `json:"freeformConfig"`
	InstanceValues    *string `json:"instanceValues"`
	NetworkName       string  `json:"networkName"`
	SerialNumber      string  `json:"serialNumber"`
	SwitchPorts       string  `json:"switchPorts"`
	TorPorts          string  `json:"torPorts"`
	Untagged          bool    `json:"untagged"`
}
