package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SharedContractAssociation defines a common contract association that every job should have
type SharedContractAssociation struct {
	DstGroupName string // Destination security group name (e.g., "ActiveDirectory")
	ContractName string // Contract name to use (e.g., "matchAD")
}

// SharedContracts is the list of common contract associations applied to every job
// Add or remove entries here to manage shared service access
var SharedContracts = []SharedContractAssociation{
	{DstGroupName: "ActiveDirectory", ContractName: "matchAD"},
}

// JobService handles job provisioning and deprovisioning
type JobService struct {
	db       *gorm.DB
	ndClient *ndclient.Client
	cfg      *config.NexusDashboardConfig

	// Cache for shared group IDs (refreshed periodically)
	sharedGroupCache     map[string]int // groupName -> groupID
	sharedGroupCacheMu   sync.RWMutex
	sharedGroupCacheTime time.Time
	sharedGroupCacheTTL  time.Duration
}

// NewJobService creates a new JobService
func NewJobService(db *gorm.DB, ndClient *ndclient.Client, cfg *config.NexusDashboardConfig) *JobService {
	return &JobService{
		db:                  db,
		ndClient:            ndClient,
		cfg:                 cfg,
		sharedGroupCache:    make(map[string]int),
		sharedGroupCacheTTL: 5 * time.Minute,
	}
}

// ProvisionInput represents the input for job provisioning
type ProvisionInput struct {
	SlurmJobID   string
	Name         string
	ComputeNodes []string
	DurationDays int
}

// ProvisionResult represents the result of job provisioning
type ProvisionResult struct {
	Job     *models.Job
	Created bool // true if new job was created, false if existing job returned
}

// portInfo holds information about a port for provisioning
type portInfo struct {
	switchPortID  string
	serialNumber  string
	interfaceName string
}

// Provision creates and provisions a new job, or returns existing job if idempotent
func (s *JobService) Provision(ctx context.Context, input ProvisionInput) (*ProvisionResult, error) {
	// Check if job already exists (idempotent)
	var existingJob models.Job
	err := s.db.WithContext(ctx).Where("slurm_job_id = ?", input.SlurmJobID).First(&existingJob).Error
	if err == nil {
		// Job exists - return it if still active/provisioning
		if existingJob.Status == string(models.JobStatusActive) ||
			existingJob.Status == string(models.JobStatusProvisioning) {
			s.db.WithContext(ctx).Preload("ComputeNodes.ComputeNode").
				Preload("SecurityGroup.Selectors.SwitchPort").
				First(&existingJob, "id = ?", existingJob.ID)
			return &ProvisionResult{Job: &existingJob, Created: false}, nil
		}
		// Job exists but completed/failed - conflict
		return nil, fmt.Errorf("job %s already exists with status %s", input.SlurmJobID, existingJob.Status)
	}
	// Distinguish "not found" from other DB errors (connection issues, context canceled, etc.)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("lookup job %s: %w", input.SlurmJobID, err)
	}

	// Use config values
	fabricName := s.cfg.ComputeFabricName
	vrfName := s.cfg.ComputeVRFName
	networkName := s.cfg.ComputeNetworkName

	// Generate contract name
	contractName := input.SlurmJobID
	if s.cfg.ComputeContractPrefix != "" {
		contractName = s.cfg.ComputeContractPrefix + "-" + input.SlurmJobID
	}

	// Start transaction for local DB operations
	var job models.Job
	var portInfos []portInfo
	var portSelectors []ndclient.NetworkPortSelector

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock compute nodes to prevent race conditions
		// Order by ID to prevent deadlocks when multiple transactions lock the same nodes
		var computeNodes []models.ComputeNode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("name IN ? OR hostname IN ?", input.ComputeNodes, input.ComputeNodes).
			Order("id").
			Find(&computeNodes).Error; err != nil {
			return fmt.Errorf("failed to lock compute nodes: %w", err)
		}

		// Verify all requested compute nodes were found
		// Handle case where input contains both name and hostname for same node (e.g., ["node1", "node1.hpc"])
		if len(computeNodes) == 0 {
			return fmt.Errorf("no compute nodes found matching: %v", input.ComputeNodes)
		}

		// Build lookup maps from found nodes (O(n) instead of O(n*m))
		nodeByName := make(map[string]string)     // name -> node ID
		nodeByHostname := make(map[string]string) // hostname -> node ID
		for _, cn := range computeNodes {
			nodeByName[cn.Name] = cn.ID
			nodeByHostname[cn.Hostname] = cn.ID
		}

		// Check which inputs resolved
		var missing []string
		for _, requested := range input.ComputeNodes {
			if _, ok := nodeByName[requested]; !ok {
				if _, ok := nodeByHostname[requested]; !ok {
					missing = append(missing, requested)
				}
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("compute nodes not found: %v", missing)
		}

		// Create job record first (needed for allocation foreign key)
		now := time.Now()
		job = models.Job{
			ID:           uuid.New().String(),
			SlurmJobID:   input.SlurmJobID,
			Name:         input.Name,
			Status:       string(models.JobStatusPending),
			FabricName:   fabricName,
			VRFName:      vrfName,
			ContractName: contractName,
			SubmittedAt:  now,
		}

		if input.DurationDays > 0 {
			expiresAt := now.AddDate(0, 0, input.DurationDays)
			job.ExpiresAt = &expiresAt
		}

		if err := tx.Create(&job).Error; err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}

		// Collect job-compute node links and port info
		jobNodes := make([]models.JobComputeNode, 0, len(computeNodes))
		for _, node := range computeNodes {
			jobNodes = append(jobNodes, models.JobComputeNode{
				ID:            uuid.New().String(),
				JobID:         job.ID,
				ComputeNodeID: node.ID,
			})

			// Get port mappings
			var mappings []models.ComputeNodePortMapping
			if err := tx.Preload("SwitchPort.Switch").
				Where("compute_node_id = ?", node.ID).
				Find(&mappings).Error; err != nil {
				return fmt.Errorf("failed to get port mappings for %s: %w", node.Name, err)
			}

			for _, mapping := range mappings {
				if mapping.SwitchPort != nil && mapping.SwitchPort.Switch != nil {
					portSelectors = append(portSelectors, ndclient.NetworkPortSelector{
						Network:       networkName,
						SwitchID:      mapping.SwitchPort.Switch.SerialNumber,
						InterfaceName: mapping.SwitchPort.Name,
					})
					portInfos = append(portInfos, portInfo{
						switchPortID:  mapping.SwitchPortID,
						serialNumber:  mapping.SwitchPort.Switch.SerialNumber,
						interfaceName: mapping.SwitchPort.Name,
					})
				}
			}
		}

		// Bulk insert job-compute node links
		if len(jobNodes) > 0 {
			if err := tx.CreateInBatches(jobNodes, 100).Error; err != nil {
				return fmt.Errorf("failed to link compute nodes: %w", err)
			}
		}

		// Allocate compute nodes - unique constraint on compute_node_id prevents double-allocation
		// This is the bulletproof concurrency check: if another job already allocated a node,
		// the insert will fail with a unique constraint violation
		allocations := make([]models.ComputeNodeAllocation, 0, len(computeNodes))
		for _, node := range computeNodes {
			allocations = append(allocations, models.ComputeNodeAllocation{
				ID:            uuid.New().String(),
				ComputeNodeID: node.ID,
				JobID:         job.ID,
				AllocatedAt:   now,
			})
		}
		if len(allocations) > 0 {
			if err := tx.Create(&allocations).Error; err != nil {
				// Check if this is a unique constraint violation (node already allocated)
				// Query which nodes are already allocated by OTHER jobs
				var conflicts []struct {
					NodeName   string
					NodeID     string
					JobSlurmID string
				}
				q := tx.Raw(`
					SELECT cn.name as node_name, cn.id as node_id, j.slurm_job_id as job_slurm_id
					FROM compute_node_allocations a
					JOIN compute_nodes cn ON cn.id = a.compute_node_id
					JOIN jobs j ON j.id = a.job_id
					WHERE a.compute_node_id IN ?
					AND a.job_id <> ?
				`, nodeIDs(computeNodes), job.ID).Scan(&conflicts)
				if q.Error != nil {
					return fmt.Errorf("failed to determine allocation conflicts: %w", q.Error)
				}

				if len(conflicts) > 0 {
					var conflictMsgs []string
					for _, c := range conflicts {
						conflictMsgs = append(conflictMsgs, fmt.Sprintf("%s [%s] (job %s)", c.NodeName, c.NodeID, c.JobSlurmID))
					}
					return fmt.Errorf("compute nodes already allocated: %v", conflictMsgs)
				}
				// If no conflicts found, it's a different DB error
				return fmt.Errorf("failed to allocate compute nodes: %w", err)
			}
		}

		// Update status to provisioning
		job.Status = string(models.JobStatusProvisioning)
		if err := tx.Save(&job).Error; err != nil {
			return fmt.Errorf("failed to update job status: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Now do NDFC provisioning (outside transaction)
	if err := s.provisionNDFC(ctx, &job, portInfos, portSelectors, fabricName, vrfName, networkName, input.SlurmJobID); err != nil {
		// Mark job as failed and release allocations to allow retry with same nodes
		job.Status = string(models.JobStatusFailed)
		errMsg := err.Error()
		job.ErrorMessage = &errMsg
		s.db.WithContext(ctx).Save(&job)

		// Release allocations so nodes can be used by retry or other jobs
		s.db.WithContext(ctx).Where("job_id = ?", job.ID).Delete(&models.ComputeNodeAllocation{})
		logger.Warn("Released compute node allocations after provisioning failure",
			zap.String("job_id", job.ID),
			zap.String("slurm_job_id", input.SlurmJobID))

		return nil, fmt.Errorf("NDFC provisioning failed: %w", err)
	}

	// Load full job with relations
	s.db.WithContext(ctx).Preload("ComputeNodes.ComputeNode").
		Preload("SecurityGroup.Selectors.SwitchPort").
		First(&job, "id = ?", job.ID)

	logger.Info("Job provisioned successfully",
		zap.String("job_id", job.ID),
		zap.String("slurm_job_id", input.SlurmJobID),
	)

	return &ProvisionResult{Job: &job, Created: true}, nil
}

// NDFC timeout constants
const (
	ndfcProvisionTimeout   = 10 * time.Minute // Overall provisioning timeout
	ndfcInterfaceTimeout   = 3 * time.Minute  // Per-step: interface config + deploy + attach
	ndfcSecurityTimeout    = 30 * time.Second // Per-step: SG/contract/association operations
	ndfcDeprovisionTimeout = 5 * time.Minute  // Overall deprovisioning timeout
)

// provisionNDFC handles all NDFC provisioning steps
func (s *JobService) provisionNDFC(ctx context.Context, job *models.Job, portInfos []portInfo, portSelectors []ndclient.NetworkPortSelector, fabricName, vrfName, networkName, slurmJobID string) error {
	if s.ndClient == nil {
		return nil
	}

	// Apply overall timeout for provisioning
	ctx, cancel := context.WithTimeout(ctx, ndfcProvisionTimeout)
	defer cancel()

	// 1. Configure and attach ports to network (with dedicated timeout)
	ifCtx, ifCancel := context.WithTimeout(ctx, ndfcInterfaceTimeout)
	err := s.configureInterfaces(ifCtx, portInfos, fabricName, networkName, slurmJobID)
	ifCancel()
	if err != nil {
		return fmt.Errorf("interface configuration failed: %w", err)
	}

	// 2. Create security group (idempotent: treat "already exists" as success)
	groupName := fmt.Sprintf("job-%s", slurmJobID)
	groupID := s.generateGroupID(slurmJobID)

	// Dedupe port selectors before sending to NDFC
	portSelectors = dedupePortSelectors(portSelectors)

	securityGroup := &ndclient.SecurityGroup{
		FabricName:           fabricName,
		GroupID:              &groupID,
		GroupName:            groupName,
		Attach:               true,
		NetworkPortSelectors: portSelectors,
	}

	// Create security group with dedicated timeout
	sgCtx, sgCancel := context.WithTimeout(ctx, ndfcSecurityTimeout)
	_, err = s.ndClient.CreateSecurityGroup(sgCtx, fabricName, securityGroup)
	if err != nil && !ndclient.IsConflictError(err) {
		sgCancel()
		return fmt.Errorf("failed to create security group: %w", err)
	}

	// Always fetch the group after create (success or conflict) to get the real NDFC-assigned ID
	// This handles cases where NDFC returns success but with nil GroupID, or assigns a different ID
	fetchedGroup, fetchErr := s.ndClient.GetSecurityGroupByName(sgCtx, fabricName, groupName)
	sgCancel()
	if fetchErr != nil {
		return fmt.Errorf("failed to fetch security group after create: %w", fetchErr)
	}
	if fetchedGroup.GroupID != nil {
		groupID = *fetchedGroup.GroupID
	}
	logger.Info("Security group ready in NDFC", zap.String("group", groupName), zap.Int("groupId", groupID))

	// 3. Save local security group, selectors, and update job in a transaction (idempotent)
	// Use OnConflict upsert - single query, no race window
	localGroup := models.SecurityGroup{
		ID:         uuid.New().String(),
		Name:       groupName,
		FabricName: fabricName,
		NDObjectID: fmt.Sprintf("%d", groupID),
	}

	// Wrap local DB writes in transaction for consistency
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert: insert or update NDObjectID on conflict (fabric_name, name)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fabric_name"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"nd_object_id", "updated_at"}),
		}).Create(&localGroup).Error; err != nil {
			return fmt.Errorf("failed to upsert local security group: %w", err)
		}

		// Fetch the actual record to get the ID (may be existing or newly created)
		if err := tx.Where("fabric_name = ? AND name = ?", fabricName, groupName).First(&localGroup).Error; err != nil {
			return fmt.Errorf("failed to fetch local security group: %w", err)
		}

		// Upsert selectors in batch: use OnConflict to handle duplicates on (security_group_id, switch_port_id)
		if len(portInfos) > 0 {
			selectors := make([]models.PortSelector, 0, len(portInfos))
			for _, pi := range portInfos {
				selectors = append(selectors, models.PortSelector{
					ID:              uuid.New().String(),
					SecurityGroupID: localGroup.ID,
					SwitchPortID:    pi.switchPortID,
					Expression:      fmt.Sprintf("%s:%s", pi.serialNumber, pi.interfaceName),
				})
			}
			// OnConflict: if (security_group_id, switch_port_id) exists, update expression
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "security_group_id"}, {Name: "switch_port_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"expression", "updated_at"}),
			}).CreateInBatches(selectors, 100).Error; err != nil {
				return fmt.Errorf("failed to upsert port selectors: %w", err)
			}
		}

		job.SecurityGroupID = &localGroup.ID
		job.Status = string(models.JobStatusActive)
		provisionedAt := time.Now()
		job.ProvisionedAt = &provisionedAt
		job.ErrorMessage = nil // Clear any previous error
		return tx.Save(job).Error
	}); err != nil {
		return fmt.Errorf("failed to save local state: %w", err)
	}

	// 6. Create contract and associations (best-effort, with dedicated timeout)
	secCtx, secCancel := context.WithTimeout(ctx, ndfcSecurityTimeout)
	s.createContractAndAssociations(secCtx, fabricName, vrfName, job.ContractName, groupName, groupID)
	secCancel()

	return nil
}

// configureInterfaces configures interfaces with int_access_host policy and attaches to network
// 1. Query network VLAN from NDFC
// 2. Configure interface settings (access mode, VLAN, PFC, QoS, etc.) via int_access_host policy
// 3. Deploy interface configurations
// 4. Attach ports to network
func (s *JobService) configureInterfaces(ctx context.Context, portInfos []portInfo, fabricName, networkName, slurmJobID string) error {
	if len(portInfos) == 0 {
		return nil
	}

	// Dedupe ports by (serialNumber, interfaceName) to avoid duplicate NDFC calls
	portInfos = dedupePortInfos(portInfos)

	// Query the network's VLAN from NDFC - fail if not found
	accessVlan, err := s.ndClient.LANFabric().GetNetworkVLAN(ctx, fabricName, networkName)
	if err != nil {
		return fmt.Errorf("failed to get VLAN for network %s: %w", networkName, err)
	}

	logger.Info("Retrieved network VLAN",
		zap.String("network", networkName),
		zap.String("vlan", accessVlan))

	// Group interfaces by switch for batch deploy
	interfacesBySwitch := make(map[string][]string)

	// 1. Configure each interface with int_access_host policy (access mode, VLAN, PFC, QoS, etc.)
	for _, pi := range portInfos {
		err := s.ndClient.LANFabric().ConfigureAccessHostInterface(
			ctx,
			pi.serialNumber,
			pi.interfaceName,
			accessVlan,
			fmt.Sprintf("HPC Job %s", slurmJobID),
		)
		if err != nil {
			logger.Warn("Failed to configure interface",
				zap.String("switch", pi.serialNumber),
				zap.String("interface", pi.interfaceName),
				zap.Error(err))
		} else {
			interfacesBySwitch[pi.serialNumber] = append(interfacesBySwitch[pi.serialNumber], pi.interfaceName)
		}
	}

	// 2. Deploy interface configurations per switch
	for serialNumber, ifNames := range interfacesBySwitch {
		if err := s.ndClient.LANFabric().DeployInterfacesNDFC(ctx, serialNumber, ifNames); err != nil {
			logger.Warn("Failed to deploy interfaces",
				zap.String("switch", serialNumber),
				zap.Strings("interfaces", ifNames),
				zap.Error(err))
		}
	}

	// 3. Attach ports to network (NDFC derives VLAN from network definition)
	var attachments []lanfabric.NetworkAttachment
	for _, pi := range portInfos {
		attachments = append(attachments, lanfabric.NetworkAttachment{
			Deployment:   true,
			Dot1QVlan:    1, // Required field, but NDFC uses network's VLAN
			Fabric:       fabricName,
			NetworkName:  networkName,
			SerialNumber: pi.serialNumber,
			SwitchPorts:  pi.interfaceName,
			Untagged:     false,
		})
	}

	if err := s.ndClient.LANFabric().AttachPortsToNetwork(ctx, fabricName, networkName, attachments); err != nil {
		return fmt.Errorf("failed to attach ports to network %s: %w", networkName, err)
	}

	logger.Info("Configured and attached ports to network",
		zap.String("network", networkName),
		zap.String("job", slurmJobID),
		zap.Int("port_count", len(attachments)))

	return nil
}

// createContractAndAssociations creates the security contract and associations (idempotent)
func (s *JobService) createContractAndAssociations(ctx context.Context, fabricName, vrfName, contractName, groupName string, groupID int) {
	// Create contract (idempotent: conflict = already exists = success)
	contract := &ndclient.SecurityContract{
		ContractName: contractName,
		Rules: []ndclient.ContractRule{
			{Direction: "bidirectional", Action: "permit", ProtocolName: "icmp"},
		},
	}
	if _, err := s.ndClient.CreateSecurityContract(ctx, fabricName, contract); err != nil {
		if !ndclient.IsConflictError(err) {
			logger.Warn("Failed to create security contract", zap.Error(err))
		}
	}

	// Create self-referential association (idempotent: conflict = already exists = success)
	association := &ndclient.ContractAssociation{
		FabricName:   fabricName,
		VRFName:      vrfName,
		SrcGroupID:   &groupID,
		DstGroupID:   &groupID,
		SrcGroupName: groupName,
		DstGroupName: groupName,
		ContractName: contractName,
		Attach:       true,
	}
	if _, err := s.ndClient.CreateSecurityAssociation(ctx, fabricName, association); err != nil {
		if !ndclient.IsConflictError(err) {
			logger.Warn("Failed to create contract association", zap.Error(err))
		}
	}

	// Create shared contract associations
	s.createSharedAssociations(ctx, fabricName, vrfName, groupName, groupID)
}

// createSharedAssociations creates associations for shared services
func (s *JobService) createSharedAssociations(ctx context.Context, fabricName, vrfName, groupName string, groupID int) {
	if len(SharedContracts) == 0 {
		return
	}

	groupIDMap := s.getSharedGroupIDs(ctx, fabricName)

	for _, shared := range SharedContracts {
		dstGroupID, found := groupIDMap[shared.DstGroupName]
		if !found {
			logger.Warn("Shared service security group not found",
				zap.String("group_name", shared.DstGroupName))
			continue
		}

		association := &ndclient.ContractAssociation{
			FabricName:   fabricName,
			VRFName:      vrfName,
			SrcGroupID:   &groupID,
			SrcGroupName: groupName,
			DstGroupID:   &dstGroupID,
			DstGroupName: shared.DstGroupName,
			ContractName: shared.ContractName,
			Attach:       true,
		}

		if _, err := s.ndClient.CreateSecurityAssociation(ctx, fabricName, association); err != nil {
			// Conflict = already exists = success (idempotent)
			if !ndclient.IsConflictError(err) {
				logger.Warn("Failed to create shared contract association",
					zap.String("dst_group", shared.DstGroupName),
					zap.String("contract", shared.ContractName),
					zap.Error(err))
			}
		} else {
			logger.Info("Created shared contract association",
				zap.String("src_group", groupName),
				zap.String("dst_group", shared.DstGroupName),
				zap.String("contract", shared.ContractName))
		}
	}
}

// getSharedGroupIDs returns cached shared group IDs, refreshing if needed
// Returns a COPY of the cache to prevent data races from callers mutating it
func (s *JobService) getSharedGroupIDs(ctx context.Context, fabricName string) map[string]int {
	s.sharedGroupCacheMu.RLock()
	if time.Since(s.sharedGroupCacheTime) < s.sharedGroupCacheTTL && len(s.sharedGroupCache) > 0 {
		out := copyStringIntMap(s.sharedGroupCache)
		s.sharedGroupCacheMu.RUnlock()
		return out
	}
	s.sharedGroupCacheMu.RUnlock()

	// Refresh cache
	s.sharedGroupCacheMu.Lock()
	defer s.sharedGroupCacheMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(s.sharedGroupCacheTime) < s.sharedGroupCacheTTL && len(s.sharedGroupCache) > 0 {
		return copyStringIntMap(s.sharedGroupCache)
	}

	allGroups, err := s.ndClient.GetSecurityGroups(ctx, fabricName)
	if err != nil {
		logger.Warn("Failed to refresh shared group cache", zap.Error(err))
		return copyStringIntMap(s.sharedGroupCache) // Return copy of stale cache
	}

	newCache := make(map[string]int)
	for _, g := range allGroups {
		if g.GroupID != nil {
			newCache[g.GroupName] = *g.GroupID
		}
	}

	s.sharedGroupCache = newCache
	s.sharedGroupCacheTime = time.Now()
	return copyStringIntMap(newCache)
}

// copyStringIntMap returns a shallow copy of a map to prevent data races
func copyStringIntMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Deprovision cleans up NDFC resources for a job
// This is the unified cleanup function used by CompleteJob and CleanupExpiredJobs
func (s *JobService) Deprovision(ctx context.Context, job *models.Job) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	// Ensure SecurityGroup is loaded (don't depend on caller preloading)
	if job.SecurityGroupID != nil && job.SecurityGroup == nil {
		var sg models.SecurityGroup
		if err := s.db.WithContext(ctx).First(&sg, "id = ?", *job.SecurityGroupID).Error; err == nil {
			job.SecurityGroup = &sg
		}
	}

	// Update status to deprovisioning
	job.Status = string(models.JobStatusDeprovisioning)
	if err := s.db.WithContext(ctx).Save(job).Error; err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Cleanup NDFC resources
	var ndfcError error
	if s.ndClient != nil && job.SecurityGroup != nil {
		ndfcError = s.deprovisionNDFC(ctx, job)
	}

	// Always release local resources regardless of NDFC cleanup result
	// This prevents nodes from being stranded when NDFC has transient issues
	// Wrap all local cleanup in a transaction for atomicity
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete local security group and selectors
		if job.SecurityGroupID != nil {
			if err := tx.Where("security_group_id = ?", *job.SecurityGroupID).Delete(&models.PortSelector{}).Error; err != nil {
				return fmt.Errorf("failed to delete port selectors: %w", err)
			}
			if err := tx.Delete(&models.SecurityGroup{}, "id = ?", *job.SecurityGroupID).Error; err != nil {
				return fmt.Errorf("failed to delete security group: %w", err)
			}
		}

		// Release compute node allocations (allows nodes to be used by other jobs)
		if err := tx.Where("job_id = ?", job.ID).Delete(&models.ComputeNodeAllocation{}).Error; err != nil {
			return fmt.Errorf("failed to release allocations: %w", err)
		}

		// Update job status based on NDFC cleanup result
		if ndfcError != nil {
			job.Status = string(models.JobStatusCleanupFailed)
			errMsg := ndfcError.Error()
			job.ErrorMessage = &errMsg
		} else {
			completedAt := time.Now()
			job.CompletedAt = &completedAt
			job.Status = string(models.JobStatusCompleted)
		}
		return tx.Save(job).Error
	}); err != nil {
		return fmt.Errorf("failed to complete local cleanup: %w", err)
	}

	// If NDFC cleanup failed, log and return error after local cleanup succeeded
	if ndfcError != nil {
		logger.Warn("NDFC cleanup failed but local resources released",
			zap.String("job_id", job.ID),
			zap.Error(ndfcError))
		return ndfcError
	}

	logger.Info("Job deprovisioned",
		zap.String("job_id", job.ID),
		zap.String("slurm_job_id", job.SlurmJobID))

	return nil
}

// deprovisionNDFC handles NDFC cleanup in the correct order
// Treats 404 (Not Found) as success for idempotent deletes
func (s *JobService) deprovisionNDFC(ctx context.Context, job *models.Job) error {
	// Apply timeout to prevent hung NDFC calls from blocking indefinitely
	ctx, cancel := context.WithTimeout(ctx, ndfcDeprovisionTimeout)
	defer cancel()

	groupID, _ := strconv.Atoi(job.SecurityGroup.NDObjectID)
	if groupID <= 0 {
		return nil
	}

	// 1. Delete self-referential contract association (404 = already deleted = success)
	if job.ContractName != "" {
		if err := s.ndClient.DeleteSecurityAssociation(ctx, job.FabricName, job.VRFName, groupID, groupID, job.ContractName); err != nil {
			if !ndclient.IsNotFoundError(err) {
				logger.Warn("Failed to delete contract association", zap.Error(err))
			}
		}
	}

	// 2. Delete shared contract associations (404 = already deleted = success)
	if len(SharedContracts) > 0 {
		groupIDMap := s.getSharedGroupIDs(ctx, job.FabricName)
		for _, shared := range SharedContracts {
			dstGroupID, found := groupIDMap[shared.DstGroupName]
			if !found {
				continue
			}
			if err := s.ndClient.DeleteSecurityAssociation(ctx, job.FabricName, job.VRFName, groupID, dstGroupID, shared.ContractName); err != nil {
				if !ndclient.IsNotFoundError(err) {
					logger.Warn("Failed to delete shared contract association",
						zap.String("dst_group", shared.DstGroupName),
						zap.String("contract", shared.ContractName),
						zap.Error(err))
				}
			}
		}
	}

	// 3. Delete security contract (404 = already deleted = success)
	if job.ContractName != "" {
		if err := s.ndClient.DeleteSecurityContract(ctx, job.FabricName, job.ContractName); err != nil {
			if !ndclient.IsNotFoundError(err) {
				logger.Warn("Failed to delete security contract", zap.Error(err))
			}
		}
	}

	// 4. Delete security group (404 = already deleted = success, other errors are fatal)
	if err := s.ndClient.DeleteSecurityGroup(ctx, job.FabricName, groupID); err != nil {
		if !ndclient.IsNotFoundError(err) {
			return fmt.Errorf("failed to delete security group: %w", err)
		}
	}

	return nil
}

// generateGroupID generates a group ID in valid range (16-65535) from job ID
func (s *JobService) generateGroupID(slurmJobID string) int {
	var groupID int
	for _, c := range slurmJobID {
		groupID = (groupID*31 + int(c)) % (65535 - 16)
	}
	return groupID + 16
}

// GetJob retrieves a job by Slurm job ID
func (s *JobService) GetJob(ctx context.Context, slurmJobID string) (*models.Job, error) {
	var job models.Job
	err := s.db.WithContext(ctx).
		Preload("ComputeNodes.ComputeNode").
		Preload("SecurityGroup.Selectors.SwitchPort").
		Where("slurm_job_id = ?", slurmJobID).
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListJobs lists jobs with optional status filter
func (s *JobService) ListJobs(ctx context.Context, status string) ([]models.Job, error) {
	query := s.db.WithContext(ctx).
		Preload("ComputeNodes.ComputeNode").
		Preload("SecurityGroup.Selectors.SwitchPort")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var jobs []models.Job
	if err := query.Order("submitted_at DESC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// CleanupExpiredJobs finds and deprovisions expired jobs
func (s *JobService) CleanupExpiredJobs(ctx context.Context) ([]string, error) {
	var expiredJobs []models.Job
	now := time.Now()

	if err := s.db.WithContext(ctx).
		Preload("SecurityGroup.Selectors").
		Where("status = ? AND expires_at < ?", models.JobStatusActive, now).
		Find(&expiredJobs).Error; err != nil {
		return nil, err
	}

	var cleaned []string
	for _, job := range expiredJobs {
		if err := s.Deprovision(ctx, &job); err != nil {
			logger.Warn("Failed to cleanup expired job",
				zap.String("slurm_job_id", job.SlurmJobID),
				zap.Error(err))
			continue
		}
		cleaned = append(cleaned, job.SlurmJobID)
	}

	return cleaned, nil
}

// Helper to extract node IDs
func nodeIDs(nodes []models.ComputeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// dedupePortInfos removes duplicate ports by (serialNumber, interfaceName)
func dedupePortInfos(portInfos []portInfo) []portInfo {
	seen := make(map[string]bool)
	result := make([]portInfo, 0, len(portInfos))
	for _, pi := range portInfos {
		key := pi.serialNumber + ":" + pi.interfaceName
		if !seen[key] {
			seen[key] = true
			result = append(result, pi)
		}
	}
	return result
}

// dedupePortSelectors removes duplicate selectors by (SwitchID, InterfaceName)
func dedupePortSelectors(selectors []ndclient.NetworkPortSelector) []ndclient.NetworkPortSelector {
	seen := make(map[string]bool)
	result := make([]ndclient.NetworkPortSelector, 0, len(selectors))
	for _, s := range selectors {
		key := s.SwitchID + ":" + s.InterfaceName
		if !seen[key] {
			seen[key] = true
			result = append(result, s)
		}
	}
	return result
}
