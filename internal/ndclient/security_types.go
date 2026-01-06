package ndclient

// Security Group types matching NDFC API (single type for request/response)

type SecurityGroup struct {
	FabricName              string                   `json:"fabricName,omitempty"`
	SourceFabric            string                   `json:"sourceFabric,omitempty"`
	GroupID                 *int                     `json:"groupId,omitempty"` // pointer so nil is omitted on create
	GroupName               string                   `json:"groupName"`
	Attach                  bool                     `json:"attach"`
	IPSelectors             []IPSelector             `json:"ipSelectors,omitempty"`
	NetworkSelectors        []NetworkSelector        `json:"networkSelectors,omitempty"`
	NetworkPortSelectors    []NetworkPortSelector    `json:"networkPortSelectors,omitempty"`
	VMInstanceUUIDSelectors []VMInstanceUUIDSelector `json:"vmInstanceUUIDSelectors,omitempty"`
}

// IPSelector type constants per NDFC API spec
const (
	IPSelectorTypeConnectedEndpoints = "Connected Endpoints"
	IPSelectorTypeExternalSubnets    = "External Subnets"
)

type IPSelector struct {
	Type       string `json:"type"`         // "Connected Endpoints" or "External Subnets"
	IP         string `json:"ip,omitempty"` // IP address or subnet (e.g., "10.101.0.50" or "20.101.10.0/24")
	VRFName    string `json:"vrfName"`
	CreatedBy  string `json:"createdBy,omitempty"`
	DeletedStr string `json:"deletedStr,omitempty"`
}

type NetworkSelector struct {
	VRFName string `json:"vrfName"`
	Network string `json:"network"`
}

type NetworkPortSelector struct {
	Network       string `json:"network"`
	SwitchID      string `json:"switchId"`
	InterfaceName string `json:"interfaceName"` // Must be full format (e.g., "Ethernet1/5", not "Eth1/5")
}

type VMInstanceUUIDSelector struct {
	VCenter        string `json:"vCenter,omitempty"`
	VMUUID         string `json:"vmUUID,omitempty"`
	VMNicMac       string `json:"vmNicMac,omitempty"`
	VMNicName      string `json:"vmNicName,omitempty"`
	VLAN           string `json:"vlan,omitempty"`
	IP             string `json:"ip,omitempty"`
	Network        string `json:"network,omitempty"`
	VRFName        string `json:"vrfName,omitempty"`
	Host           string `json:"host,omitempty"`
	Switch         string `json:"switch,omitempty"`
	SwitchIntf     string `json:"switchIntf,omitempty"`
	Name           string `json:"name,omitempty"`
	ConfiguredSgid string `json:"configuredSgid,omitempty"`
	Type           string `json:"type,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
	DeletedStr     string `json:"deletedStr,omitempty"`
}

// Security Protocol types matching NDFC API (single type for request/response)

type SecurityProtocol struct {
	FabricName   string              `json:"fabricName,omitempty"`
	ProtocolName string              `json:"protocolName"`
	Description  string              `json:"description,omitempty"`
	MatchType    string              `json:"matchType"` // "any" or "all"
	MatchItems   []ProtocolMatchItem `json:"matchItems,omitempty"`
}

type ProtocolMatchItem struct {
	Type            string `json:"type"`                      // "Default"
	ProtocolOptions string `json:"protocolOptions,omitempty"` // "TCP", "UDP", "ICMP", etc.
	SrcPortRange    string `json:"srcPortRange,omitempty"`
	DstPortRange    string `json:"dstPortRange,omitempty"`
	TCPFlags        string `json:"tcpFlags,omitempty"` // "ack", "syn", etc.
	OnlyFragments   bool   `json:"onlyFragments"`
	Stateful        bool   `json:"stateful"`
}

// Security Contract types matching NDFC API (single type for request/response)

type SecurityContract struct {
	ContractName string         `json:"contractName"`
	Rules        []ContractRule `json:"rules,omitempty"`
}

type ContractRule struct {
	Direction    string `json:"direction"`
	Action       string `json:"action"`
	ProtocolName string `json:"protocolName,omitempty"`
}

// Contract Association types matching NDFC API (single type for request/response)

type ContractAssociation struct {
	FabricName   string `json:"fabricName,omitempty"`
	VRFName      string `json:"vrfName"`
	SrcGroupID   *int   `json:"srcGroupId,omitempty"` // pointer so 0 is not sent when using names
	DstGroupID   *int   `json:"dstGroupId,omitempty"` // pointer so 0 is not sent when using names
	SrcGroupName string `json:"srcGroupName,omitempty"`
	DstGroupName string `json:"dstGroupName,omitempty"`
	ContractName string `json:"contractName"`
	Attach       bool   `json:"attach"` // always sent (NDFC requires it)
}

// BatchResponse represents NDFC batch operation response (generic)
type BatchResponse struct {
	TotalCount   int         `json:"totalCount"`
	FailedCount  int         `json:"failedCount"`
	SuccessCount int         `json:"successCount"`
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	FailureList  []BatchItem `json:"failureList"`
}

// BatchResponseGroups for security group operations
type BatchResponseGroups struct {
	BatchResponse
	SuccessList []SecurityGroup `json:"successList"`
}

// BatchResponseContracts for security contract operations
type BatchResponseContracts struct {
	BatchResponse
	SuccessList []SecurityContract `json:"successList"`
}

// BatchResponseAssociations for contract association operations
type BatchResponseAssociations struct {
	BatchResponse
	SuccessList []ContractAssociation `json:"successList"`
}

type BatchItem struct {
	ResourceType   string `json:"resourceType"`
	ResourceID     string `json:"resourceId"`
	Status         string `json:"status"`
	Name           string `json:"name"`
	AuditLogAction string `json:"auditLogAction"`
	Code           string `json:"code"`
	Message        string `json:"message"`
}
