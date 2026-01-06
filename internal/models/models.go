package models

import (
	"time"

	"gorm.io/gorm"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending        JobStatus = "pending"
	JobStatusProvisioning   JobStatus = "provisioning"
	JobStatusActive         JobStatus = "active"
	JobStatusDeprovisioning JobStatus = "deprovisioning"
	JobStatusCompleted      JobStatus = "completed"
	JobStatusCleanupFailed  JobStatus = "cleanup_failed"
	JobStatusFailed         JobStatus = "failed"
)

// IsTerminal returns true if the job is in a terminal state
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed
}

// IsActive returns true if the job is currently active or being provisioned
func (s JobStatus) IsActive() bool {
	return s == JobStatusActive || s == JobStatusProvisioning
}

// Fabric represents a Nexus Dashboard fabric
type Fabric struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null" json:"name"`
	Type      string         `json:"type"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Switches  []Switch       `gorm:"foreignKey:FabricID" json:"switches,omitempty"`
}

// Switch represents a network switch in the fabric
type Switch struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"not null" json:"name"`
	SerialNumber string         `gorm:"uniqueIndex" json:"serial_number"`
	Model        string         `json:"model"`
	IPAddress    string         `json:"ip_address"`
	FabricID     string         `gorm:"index;not null" json:"fabric_id"`
	Fabric       *Fabric        `gorm:"foreignKey:FabricID" json:"fabric,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Ports        []SwitchPort   `gorm:"foreignKey:SwitchID" json:"ports,omitempty"`
}

// SwitchPort represents a port on a switch
type SwitchPort struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null;uniqueIndex:idx_switch_port" json:"name"`
	PortNumber  string         `json:"port_number"`
	Description string         `json:"description"`
	AdminState  string         `json:"admin_state"` // NDFC admin state: "true"=enabled, "false"=disabled
	Speed       string         `json:"speed"`
	IsPresent   bool           `gorm:"default:true" json:"is_present"` // false if not seen in recent sync
	SwitchID    string         `gorm:"index;not null;uniqueIndex:idx_switch_port" json:"switch_id"`
	Switch      *Switch        `gorm:"foreignKey:SwitchID" json:"switch,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	LastSeenAt  *time.Time     `json:"last_seen_at,omitempty"`
}

// ComputeNode represents a server/compute node
type ComputeNode struct {
	ID           string                   `gorm:"primaryKey" json:"id"`
	Name         string                   `gorm:"uniqueIndex;not null" json:"name"`
	Hostname     string                   `json:"hostname"`
	IPAddress    string                   `json:"ip_address"`
	MACAddress   string                   `json:"mac_address"`
	Description  string                   `json:"description"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	DeletedAt    gorm.DeletedAt           `gorm:"index" json:"-"`
	PortMappings []ComputeNodePortMapping `gorm:"foreignKey:ComputeNodeID" json:"port_mappings,omitempty"`
}

// ComputeNodePortMapping maps a compute node to a switch port
type ComputeNodePortMapping struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	ComputeNodeID string         `gorm:"index;not null" json:"compute_node_id"`
	ComputeNode   *ComputeNode   `gorm:"foreignKey:ComputeNodeID" json:"compute_node,omitempty"`
	SwitchPortID  string         `gorm:"index;not null" json:"switch_port_id"`
	SwitchPort    *SwitchPort    `gorm:"foreignKey:SwitchPortID" json:"switch_port,omitempty"`
	NICName       string         `json:"nic_name"`
	VLAN          int            `json:"vlan"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// SecurityGroup represents a Nexus Dashboard Security Group
type SecurityGroup struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null;uniqueIndex:idx_sg_fabric_name" json:"name"`
	Description string         `json:"description"`
	NDObjectID  string         `json:"nd_object_id"`                                               // NDFC group ID (numeric string)
	FabricName  string         `gorm:"not null;uniqueIndex:idx_sg_fabric_name" json:"fabric_name"` // NDFC fabric name for API calls
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Selectors   []PortSelector `gorm:"foreignKey:SecurityGroupID" json:"selectors,omitempty"`
}

// PortSelector represents a port selector for a security group
type PortSelector struct {
	ID              string         `gorm:"primaryKey" json:"id"`
	SecurityGroupID string         `gorm:"not null;uniqueIndex:idx_ps_sg_port" json:"security_group_id"`
	SecurityGroup   *SecurityGroup `gorm:"foreignKey:SecurityGroupID" json:"security_group,omitempty"`
	SwitchPortID    string         `gorm:"uniqueIndex:idx_ps_sg_port" json:"switch_port_id"`
	SwitchPort      *SwitchPort    `gorm:"foreignKey:SwitchPortID" json:"switch_port,omitempty"`
	Expression      string         `json:"expression"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// SecurityContract represents a Nexus Dashboard Security Contract
type SecurityContract struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"uniqueIndex;not null" json:"name"` // NDFC contract name
	Description string         `json:"description"`
	NDObjectID  string         `json:"nd_object_id"`             // Same as Name for contracts
	FabricName  string         `gorm:"index" json:"fabric_name"` // NDFC fabric name for API calls
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Rules       []ContractRule `gorm:"foreignKey:SecurityContractID" json:"rules,omitempty"`
}

// ContractRule represents a rule within a security contract
type ContractRule struct {
	ID                 string            `gorm:"primaryKey" json:"id"`
	SecurityContractID string            `gorm:"index;not null" json:"security_contract_id"`
	SecurityContract   *SecurityContract `gorm:"foreignKey:SecurityContractID" json:"security_contract,omitempty"`
	Name               string            `json:"name"`
	Action             string            `json:"action"`
	Protocol           string            `json:"protocol"`
	SrcPort            string            `json:"src_port"`
	DstPort            string            `json:"dst_port"`
	Priority           int               `json:"priority"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"-"`
}

// SecurityAssociation represents a Nexus Dashboard Security Association
type SecurityAssociation struct {
	ID                 string            `gorm:"primaryKey" json:"id"`
	Name               string            `gorm:"uniqueIndex;not null" json:"name"` // Local display name
	Description        string            `json:"description"`
	FabricName         string            `gorm:"index" json:"fabric_name"` // NDFC fabric name for API calls
	VRFName            string            `json:"vrf_name"`                 // NDFC VRF name (required for delete)
	ContractName       string            `json:"contract_name"`            // NDFC contract name (required for delete)
	SrcGroupNDID       int               `json:"src_group_nd_id"`          // NDFC source group ID (required for delete)
	DstGroupNDID       int               `json:"dst_group_nd_id"`          // NDFC dest group ID (required for delete)
	ProviderGroupID    string            `gorm:"index" json:"provider_group_id"`
	ProviderGroup      *SecurityGroup    `gorm:"foreignKey:ProviderGroupID" json:"provider_group,omitempty"`
	ConsumerGroupID    string            `gorm:"index" json:"consumer_group_id"`
	ConsumerGroup      *SecurityGroup    `gorm:"foreignKey:ConsumerGroupID" json:"consumer_group,omitempty"`
	SecurityContractID string            `gorm:"index" json:"security_contract_id"`
	SecurityContract   *SecurityContract `gorm:"foreignKey:SecurityContractID" json:"security_contract,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"-"`
}

// Job represents a Slurm job with associated security provisioning
type Job struct {
	ID              string           `gorm:"primaryKey" json:"id"`
	SlurmJobID      string           `gorm:"uniqueIndex;not null" json:"slurm_job_id"`
	Name            string           `json:"name"`
	Status          string           `gorm:"index;not null" json:"status"` // pending, provisioning, active, deprovisioning, completed, failed
	ErrorMessage    *string          `json:"error_message,omitempty"`      // Error details if status is failed
	FabricName      string           `gorm:"not null" json:"fabric_name"`
	VRFName         string           `json:"vrf_name"`
	ContractName    string           `json:"contract_name"`
	SubmittedAt     time.Time        `json:"submitted_at"`
	ProvisionedAt   *time.Time       `json:"provisioned_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	ExpiresAt       *time.Time       `json:"expires_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
	ComputeNodes    []JobComputeNode `gorm:"foreignKey:JobID" json:"compute_nodes,omitempty"`
	SecurityGroupID *string          `gorm:"index" json:"security_group_id,omitempty"`
	SecurityGroup   *SecurityGroup   `gorm:"foreignKey:SecurityGroupID" json:"security_group,omitempty"`
}

// JobComputeNode links a job to the compute nodes assigned by Slurm
type JobComputeNode struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	JobID         string         `gorm:"index;not null" json:"job_id"`
	Job           *Job           `gorm:"foreignKey:JobID" json:"job,omitempty"`
	ComputeNodeID string         `gorm:"index;not null" json:"compute_node_id"`
	ComputeNode   *ComputeNode   `gorm:"foreignKey:ComputeNodeID" json:"compute_node,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// ComputeNodeAllocation tracks exclusive allocation of compute nodes to jobs.
// The unique constraint on compute_node_id ensures only one active allocation per node.
// This prevents race conditions in concurrent job provisioning.
type ComputeNodeAllocation struct {
	ID            string       `gorm:"primaryKey" json:"id"`
	ComputeNodeID string       `gorm:"uniqueIndex;not null" json:"compute_node_id"` // Only one allocation per node
	ComputeNode   *ComputeNode `gorm:"foreignKey:ComputeNodeID" json:"compute_node,omitempty"`
	JobID         string       `gorm:"index;not null" json:"job_id"`
	Job           *Job         `gorm:"foreignKey:JobID" json:"job,omitempty"`
	AllocatedAt   time.Time    `json:"allocated_at"`
}

// Tenant represents a tenant with their own VRF for VM provisioning
type Tenant struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"uniqueIndex;not null" json:"name"`
	Description string         `json:"description"`
	VRFName     string         `gorm:"not null" json:"vrf_name"` // Tenant-specific VRF for VM security
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	VMs         []VM           `gorm:"foreignKey:TenantID" json:"vms,omitempty"`
}

// VM represents a virtual machine with security provisioning
type VM struct {
	ID              string         `gorm:"primaryKey" json:"id"`
	Name            string         `gorm:"not null" json:"name"`
	TenantID        string         `gorm:"index;not null" json:"tenant_id"`
	Tenant          *Tenant        `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	VMID            string         `json:"vm_id"`                        // vCenter VM ID (MoRef)
	IPAddress       string         `json:"ip_address"`                   // VM IP for security group
	Status          string         `gorm:"index;not null" json:"status"` // pending, provisioned, deprovisioned
	SecurityGroupID string         `gorm:"index" json:"security_group_id,omitempty"`
	SecurityGroup   *SecurityGroup `gorm:"foreignKey:SecurityGroupID" json:"security_group,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
