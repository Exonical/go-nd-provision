package services

import (
	"context"
	"fmt"
	"strconv"
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

// StorageSharedContracts is the list of common contract associations applied to every storage SG
// These provide always-on access to shared services (DNS, AD, SIEM, etc.)
var StorageSharedContracts = []SharedContractAssociation{
	{DstGroupName: "SG_AD", ContractName: "AD"},
	{DstGroupName: "SG_DNS", ContractName: "DNS"},
}

// StorageService handles storage NIC provisioning and per-node storage SG management
type StorageService struct {
	db       *gorm.DB
	ndClient *ndclient.Client
	cfg      *config.NexusDashboardConfig
}

// NewStorageService creates a new StorageService
func NewStorageService(db *gorm.DB, ndClient *ndclient.Client, cfg *config.NexusDashboardConfig) *StorageService {
	return &StorageService{
		db:       db,
		ndClient: ndClient,
		cfg:      cfg,
	}
}

// storageNodeSGName returns the security group name for a node's storage interface
func storageNodeSGName(nodeName string) string {
	return fmt.Sprintf("storage-node-%s", nodeName)
}

// GetStorageNetworkName returns the configured storage network name
func (s *StorageService) GetStorageNetworkName() string {
	return s.cfg.StorageNetworkName
}

// StoragePortInfo holds information about a storage port for provisioning
type StoragePortInfo struct {
	SwitchPortID  string
	SerialNumber  string
	InterfaceName string
	NodeName      string
	NodeID        string
}

// EnsureNodeStorageSG ensures a per-node storage security group exists with correct selectors
// This is idempotent: creates if not exists, updates selectors if changed
func (s *StorageService) EnsureNodeStorageSG(ctx context.Context, node *models.ComputeNode, storagePorts []StoragePortInfo, networkName string) (int, error) {
	if s.ndClient == nil {
		return 0, nil
	}

	fabricName := s.cfg.StorageFabricName
	if fabricName == "" {
		return 0, fmt.Errorf("ND_STORAGE_FABRIC_NAME not configured")
	}

	sgName := storageNodeSGName(node.Name)

	// Build port selectors (only if we have a network name - otherwise just create empty SG)
	var portSelectors []ndclient.NetworkPortSelector
	if networkName != "" && len(storagePorts) > 0 {
		portSelectors = make([]ndclient.NetworkPortSelector, 0, len(storagePorts))
		for _, p := range storagePorts {
			portSelectors = append(portSelectors, ndclient.NetworkPortSelector{
				Network:       networkName,
				SwitchID:      p.SerialNumber,
				InterfaceName: p.InterfaceName,
			})
		}
	}

	// Check if SG already exists
	existingGroup, err := s.ndClient.GetSecurityGroupByName(ctx, fabricName, sgName)
	if err == nil && existingGroup != nil && existingGroup.GroupID != nil {
		// SG exists - update selectors
		// If we're removing all selectors, detach and clear in one call
		if len(portSelectors) == 0 {
			if existingGroup.Attach || len(existingGroup.NetworkPortSelectors) > 0 {
				// Detach and clear selectors in one call
				existingGroup.Attach = false
				existingGroup.NetworkPortSelectors = nil
				if _, err := s.ndClient.UpdateSecurityGroups(ctx, fabricName, []ndclient.SecurityGroup{*existingGroup}); err != nil {
					logger.Warn("Failed to detach and clear storage SG selectors",
						zap.String("sg", sgName),
						zap.Error(err))
				} else {
					logger.Info("Storage SG detached and selectors cleared",
						zap.String("sg", sgName),
						zap.Int("groupId", *existingGroup.GroupID))
				}
			}
		} else {
			// Normal update with selectors
			existingGroup.NetworkPortSelectors = portSelectors
			existingGroup.Attach = true
			if _, err := s.ndClient.UpdateSecurityGroups(ctx, fabricName, []ndclient.SecurityGroup{*existingGroup}); err != nil {
				logger.Warn("Failed to update storage SG selectors",
					zap.String("sg", sgName),
					zap.Error(err))
			} else {
				logger.Info("Storage SG updated",
					zap.String("sg", sgName),
					zap.String("network", networkName),
					zap.Int("groupId", *existingGroup.GroupID))
			}
		}
		logger.Info("Updated storage SG selectors",
			zap.String("node", node.Name),
			zap.Int("port_count", len(portSelectors)))
		return *existingGroup.GroupID, nil
	}

	// Create new SG
	groupID := s.generateStorageGroupID(node.Name)
	securityGroup := &ndclient.SecurityGroup{
		FabricName:           fabricName,
		GroupID:              &groupID,
		GroupName:            sgName,
		Attach:               true,
		NetworkPortSelectors: portSelectors,
	}

	_, err = s.ndClient.CreateSecurityGroup(ctx, fabricName, securityGroup)
	if err != nil && !ndclient.IsConflictError(err) {
		return 0, fmt.Errorf("failed to create storage SG %s: %w", sgName, err)
	}

	// Fetch to get actual ID
	fetchedGroup, fetchErr := s.ndClient.GetSecurityGroupByName(ctx, fabricName, sgName)
	if fetchErr != nil {
		return 0, fmt.Errorf("failed to fetch storage SG after create: %w", fetchErr)
	}
	if fetchedGroup.GroupID != nil {
		groupID = *fetchedGroup.GroupID
	}

	logger.Info("Storage SG created",
		zap.String("sg", sgName),
		zap.String("network", networkName),
		zap.Int("groupId", groupID))

	return groupID, nil
}

// EnsureStorageSharedAssociations ensures shared-services associations exist for a storage SG
func (s *StorageService) EnsureStorageSharedAssociations(ctx context.Context, sgName string, sgID int, groupIDMap map[string]int) {
	if len(StorageSharedContracts) == 0 {
		return
	}

	fabricName := s.cfg.StorageFabricName
	vrfName := s.cfg.StorageVRFName

	for _, shared := range StorageSharedContracts {
		dstGroupID, found := groupIDMap[shared.DstGroupName]
		if !found {
			logger.Warn("Shared service security group not found for storage",
				zap.String("group_name", shared.DstGroupName))
			continue
		}

		association := &ndclient.ContractAssociation{
			FabricName:   fabricName,
			VRFName:      vrfName,
			SrcGroupID:   &sgID,
			SrcGroupName: sgName,
			DstGroupID:   &dstGroupID,
			DstGroupName: shared.DstGroupName,
			ContractName: shared.ContractName,
			Attach:       true,
		}

		if _, err := s.ndClient.CreateSecurityAssociation(ctx, fabricName, association); err != nil {
			if !ndclient.IsConflictError(err) {
				logger.Warn("Failed to create storage shared contract association",
					zap.String("src_group", sgName),
					zap.String("dst_group", shared.DstGroupName),
					zap.String("contract", shared.ContractName),
					zap.Error(err))
			}
		} else {
			logger.Info("Created storage shared contract association",
				zap.String("src_group", sgName),
				zap.String("dst_group", shared.DstGroupName),
				zap.String("contract", shared.ContractName))
		}
	}
}

// ProvisionStorageForJob provisions storage access for a job's nodes
// This attaches storage ports to tenant network, updates SG selectors, and creates tenant associations
func (s *StorageService) ProvisionStorageForJob(ctx context.Context, job *models.Job, tenant *models.StorageTenant, nodes []models.ComputeNode) error {
	if s.ndClient == nil {
		return nil
	}

	fabricName := s.cfg.StorageFabricName
	vrfName := s.cfg.StorageVRFName
	baseNetworkName := s.cfg.StorageNetworkName

	if fabricName == "" || vrfName == "" {
		logger.Debug("Storage fabric/VRF not configured, skipping storage provisioning")
		return nil
	}

	// Get shared group IDs for shared-services associations
	allGroups, err := s.ndClient.GetSecurityGroups(ctx, fabricName)
	if err != nil {
		logger.Warn("Failed to get security groups for storage shared services", zap.Error(err))
	}
	groupIDMap := make(map[string]int)
	for _, g := range allGroups {
		if g.GroupID != nil {
			groupIDMap[g.GroupName] = *g.GroupID
		}
	}

	// Verify tenant destination group exists
	if _, found := groupIDMap[tenant.StorageDstGroupName]; !found {
		return fmt.Errorf("tenant storage destination group %q not found in NDFC", tenant.StorageDstGroupName)
	}

	for _, node := range nodes {
		// Get storage interface port mappings
		storagePorts, err := s.getStoragePortsForNode(ctx, &node)
		if err != nil {
			logger.Warn("Failed to get storage ports for node",
				zap.String("node", node.Name),
				zap.Error(err))
			continue
		}
		if len(storagePorts) == 0 {
			logger.Debug("Node has no storage interface mappings",
				zap.String("node", node.Name))
			continue
		}

		// 1. Attach storage ports to tenant network
		attachments := make([]lanfabric.NetworkAttachment, 0, len(storagePorts))
		for _, p := range storagePorts {
			attachments = append(attachments, lanfabric.NetworkAttachment{
				SerialNumber: p.SerialNumber,
				SwitchPorts:  p.InterfaceName,
				Deployment:   true,
			})
		}

		if err := s.ndClient.LANFabric().AttachPortsToNetwork(ctx, fabricName, tenant.StorageNetworkName, attachments); err != nil {
			logger.Warn("Failed to attach storage ports to tenant network",
				zap.String("node", node.Name),
				zap.String("network", tenant.StorageNetworkName),
				zap.Error(err))
			// Continue - try to set up SG anyway
		}

		// 2. Ensure per-node storage SG exists with selectors pointing to tenant network
		sgName := storageNodeSGName(node.Name)
		sgID, err := s.EnsureNodeStorageSG(ctx, &node, storagePorts, tenant.StorageNetworkName)
		if err != nil {
			logger.Warn("Failed to ensure storage SG for node",
				zap.String("node", node.Name),
				zap.Error(err))
			continue
		}

		// 3. Ensure shared-services associations exist (e.g., SG_AD for AD/DNS)
		s.EnsureStorageSharedAssociations(ctx, sgName, sgID, groupIDMap)

		// 4. Create association between storage SG and tenant storage network SG
		// This allows the node to access the tenant's storage network
		if tenant.StorageNetworkSGName != "" {
			tenantNetSGID, found := groupIDMap[tenant.StorageNetworkSGName]
			if !found {
				return fmt.Errorf("tenant storage network SG %q not found in NDFC", tenant.StorageNetworkSGName)
			}
			assoc := &ndclient.ContractAssociation{
				FabricName:   fabricName,
				VRFName:      vrfName,
				SrcGroupID:   &sgID,
				SrcGroupName: sgName,
				DstGroupID:   &tenantNetSGID,
				DstGroupName: tenant.StorageNetworkSGName,
				ContractName: tenant.StorageContractName,
				Attach:       true,
			}
			if _, err := s.ndClient.CreateSecurityAssociation(ctx, fabricName, assoc); err != nil {
				if !ndclient.IsConflictError(err) {
					return fmt.Errorf("failed to create tenant storage network association: %w", err)
				}
			} else {
				logger.Info("Created tenant storage network association",
					zap.String("src_group", sgName),
					zap.String("dst_group", tenant.StorageNetworkSGName),
					zap.String("contract", tenant.StorageContractName))
			}
		}

		// 5. Record what we created for cleanup
		storageAccess := models.JobStorageAccess{
			ID:              uuid.New().String(),
			JobID:           job.ID,
			ComputeNodeID:   node.ID,
			StorageTenantID: tenant.ID,
			SrcGroupName:    sgName,
			DstGroupName:    tenant.StorageDstGroupName,
			ContractName:    tenant.StorageContractName,
			FabricName:      fabricName,
			VRFName:         vrfName,
			PrevNetworkName: baseNetworkName,
			CreatedAt:       time.Now(),
		}
		if err := s.db.WithContext(ctx).Create(&storageAccess).Error; err != nil {
			logger.Warn("Failed to record storage access for cleanup",
				zap.String("node", node.Name),
				zap.Error(err))
		}

		logger.Info("Provisioned storage access for node",
			zap.String("node", node.Name),
			zap.String("tenant", tenant.Key),
			zap.String("network", tenant.StorageNetworkName))
	}

	return nil
}

// DeprovisionStorageForJob removes tenant storage access for a job
// This deletes tenant associations and reverts storage ports to base network
func (s *StorageService) DeprovisionStorageForJob(ctx context.Context, job *models.Job) error {
	if s.ndClient == nil {
		return nil
	}

	// Get all storage access records for this job
	var storageAccesses []models.JobStorageAccess
	if err := s.db.WithContext(ctx).
		Preload("ComputeNode").
		Where("job_id = ?", job.ID).
		Find(&storageAccesses).Error; err != nil {
		return fmt.Errorf("failed to get storage access records: %w", err)
	}

	if len(storageAccesses) == 0 {
		return nil
	}

	fabricName := s.cfg.StorageFabricName
	baseNetworkName := s.cfg.StorageNetworkName

	// Get all security groups to resolve IDs
	allGroups, err := s.ndClient.GetSecurityGroups(ctx, fabricName)
	if err != nil {
		logger.Warn("Failed to get security groups for storage deprovision", zap.Error(err))
	}
	groupIDMap := make(map[string]int)
	for _, g := range allGroups {
		if g.GroupID != nil {
			groupIDMap[g.GroupName] = *g.GroupID
		}
	}

	for _, access := range storageAccesses {
		srcGroupID, srcFound := groupIDMap[access.SrcGroupName]
		dstGroupID, dstFound := groupIDMap[access.DstGroupName]

		// 1. Delete tenant storage association
		if srcFound && dstFound {
			if err := s.ndClient.DeleteSecurityAssociation(ctx, access.FabricName, access.VRFName, srcGroupID, dstGroupID, access.ContractName); err != nil {
				if !ndclient.IsNotFoundError(err) {
					logger.Warn("Failed to delete tenant storage association",
						zap.String("src_group", access.SrcGroupName),
						zap.String("dst_group", access.DstGroupName),
						zap.Error(err))
				}
			}
		}

		// 2. Revert storage ports to base network
		if access.ComputeNode != nil && access.PrevNetworkName != "" && baseNetworkName != "" {
			storagePorts, err := s.getStoragePortsForNode(ctx, access.ComputeNode)
			if err == nil && len(storagePorts) > 0 {
				attachments := make([]lanfabric.NetworkAttachment, 0, len(storagePorts))
				for _, p := range storagePorts {
					attachments = append(attachments, lanfabric.NetworkAttachment{
						SerialNumber: p.SerialNumber,
						SwitchPorts:  p.InterfaceName,
						Deployment:   true,
					})
				}

				if err := s.ndClient.LANFabric().AttachPortsToNetwork(ctx, fabricName, baseNetworkName, attachments); err != nil {
					logger.Warn("Failed to revert storage ports to base network",
						zap.String("node", access.ComputeNode.Name),
						zap.String("network", baseNetworkName),
						zap.Error(err))
				}

				// 3. Update SG selectors back to base network
				if srcFound {
					s.updateStorageSGNetwork(ctx, access.ComputeNode, storagePorts, baseNetworkName)
				}
			}
		}

		// 4. Delete the tracking record
		if err := s.db.WithContext(ctx).Delete(&access).Error; err != nil {
			logger.Warn("Failed to delete storage access record",
				zap.String("id", access.ID),
				zap.Error(err))
		}

		logger.Info("Deprovisioned storage access for node",
			zap.String("node_id", access.ComputeNodeID),
			zap.String("job_id", job.ID))
	}

	return nil
}

// updateStorageSGNetwork updates a node's storage SG selectors to point to a different network
func (s *StorageService) updateStorageSGNetwork(ctx context.Context, node *models.ComputeNode, storagePorts []StoragePortInfo, networkName string) {
	fabricName := s.cfg.StorageFabricName
	sgName := storageNodeSGName(node.Name)

	existingGroup, err := s.ndClient.GetSecurityGroupByName(ctx, fabricName, sgName)
	if err != nil || existingGroup == nil {
		return
	}

	// Build new selectors with updated network
	portSelectors := make([]ndclient.NetworkPortSelector, 0, len(storagePorts))
	for _, p := range storagePorts {
		portSelectors = append(portSelectors, ndclient.NetworkPortSelector{
			Network:       networkName,
			SwitchID:      p.SerialNumber,
			InterfaceName: p.InterfaceName,
		})
	}

	existingGroup.NetworkPortSelectors = portSelectors
	existingGroup.Attach = true

	if _, err := s.ndClient.UpdateSecurityGroups(ctx, fabricName, []ndclient.SecurityGroup{*existingGroup}); err != nil {
		logger.Warn("Failed to update storage SG network",
			zap.String("sg", sgName),
			zap.String("network", networkName),
			zap.Error(err))
	}
}

// getStoragePortsForNode retrieves storage interface port mappings for a node
func (s *StorageService) getStoragePortsForNode(ctx context.Context, node *models.ComputeNode) ([]StoragePortInfo, error) {
	var mappings []models.ComputeNodePortMapping

	// Get mappings that are linked to a storage interface
	err := s.db.WithContext(ctx).
		Preload("SwitchPort.Switch").
		Preload("Interface").
		Where("compute_node_id = ?", node.ID).
		Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	var storagePorts []StoragePortInfo
	for _, m := range mappings {
		// Check if this mapping is for a storage interface
		if m.Interface != nil && m.Interface.Role == models.InterfaceRoleStorage {
			if m.SwitchPort != nil && m.SwitchPort.Switch != nil {
				storagePorts = append(storagePorts, StoragePortInfo{
					SwitchPortID:  m.SwitchPortID,
					SerialNumber:  m.SwitchPort.Switch.SerialNumber,
					InterfaceName: m.SwitchPort.Name,
					NodeName:      node.Name,
					NodeID:        node.ID,
				})
			}
		}
	}

	return storagePorts, nil
}

// generateStorageGroupID generates a group ID for storage SGs
// Range: 32768-65535 (upper half of valid range, to avoid collision with job SGs)
func (s *StorageService) generateStorageGroupID(nodeName string) int {
	var groupID int
	for _, c := range nodeName {
		groupID = (groupID*31 + int(c))
	}
	// Map to range 32768-65535 (32767 values)
	return (groupID % 32767) + 32768
}

// ReconcileNodeStorageSG ensures a node's storage SG is properly configured
// Called when node interfaces are updated or on startup
func (s *StorageService) ReconcileNodeStorageSG(ctx context.Context, node *models.ComputeNode) error {
	if s.ndClient == nil {
		return nil
	}

	fabricName := s.cfg.StorageFabricName
	baseNetworkName := s.cfg.StorageNetworkName

	if fabricName == "" || baseNetworkName == "" {
		return nil
	}

	storagePorts, err := s.getStoragePortsForNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to get storage ports: %w", err)
	}

	if len(storagePorts) == 0 {
		// No storage interface - nothing to do
		return nil
	}

	// Check if node is currently in a job with storage access
	var activeAccess models.JobStorageAccess
	err = s.db.WithContext(ctx).
		Joins("JOIN jobs ON jobs.id = job_storage_accesses.job_id").
		Where("job_storage_accesses.compute_node_id = ? AND jobs.status IN ?", node.ID, []string{"active", "provisioning"}).
		First(&activeAccess).Error

	networkName := baseNetworkName
	if err == nil {
		// Node is in an active job - use tenant network from the access record
		var tenant models.StorageTenant
		if s.db.WithContext(ctx).First(&tenant, "id = ?", activeAccess.StorageTenantID).Error == nil {
			networkName = tenant.StorageNetworkName
		}
	}

	// Ensure SG exists with correct selectors
	sgID, err := s.EnsureNodeStorageSG(ctx, node, storagePorts, networkName)
	if err != nil {
		return err
	}

	// Ensure shared-services associations
	allGroups, _ := s.ndClient.GetSecurityGroups(ctx, fabricName)
	groupIDMap := make(map[string]int)
	for _, g := range allGroups {
		if g.GroupID != nil {
			groupIDMap[g.GroupName] = *g.GroupID
		}
	}

	sgName := storageNodeSGName(node.Name)
	s.EnsureStorageSharedAssociations(ctx, sgName, sgID, groupIDMap)

	// Save local record of the storage SG
	localGroup := models.SecurityGroup{
		ID:         uuid.New().String(),
		Name:       sgName,
		FabricName: fabricName,
		NDObjectID: strconv.Itoa(sgID),
	}
	s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fabric_name"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"nd_object_id", "updated_at"}),
	}).Create(&localGroup)

	return nil
}
