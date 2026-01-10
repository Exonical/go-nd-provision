package handlers

import (
	"context"
	"net/http"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// InterfaceHandler handles compute node interface operations
type InterfaceHandler struct {
	storageService *services.StorageService
}

// NewInterfaceHandler creates a new InterfaceHandler
func NewInterfaceHandler(storageService *services.StorageService) *InterfaceHandler {
	return &InterfaceHandler{
		storageService: storageService,
	}
}

// GetInterfaces returns all interfaces for a compute node
func (h *InterfaceHandler) GetInterfaces(c *gin.Context) {
	nodeID := c.Param("id")

	// Find node by ID or name
	var node models.ComputeNode
	if err := database.DB.Where("id = ? OR name = ?", nodeID, nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	var interfaces []models.ComputeNodeInterface
	if err := database.DB.Where("compute_node_id = ?", node.ID).
		Preload("PortMappings.SwitchPort.Switch").
		Find(&interfaces).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch interfaces"})
		return
	}

	c.JSON(http.StatusOK, interfaces)
}

// CreateInterfaceInput represents the input for creating an interface
type CreateInterfaceInput struct {
	Role       string `json:"role" binding:"required,oneof=compute storage"`
	Hostname   string `json:"hostname"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
}

// CreateInterface creates a new interface for a compute node
func (h *InterfaceHandler) CreateInterface(c *gin.Context) {
	nodeID := c.Param("id")

	var input CreateInterfaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role is valid (compute or storage)
	if input.Role != string(models.InterfaceRoleCompute) && input.Role != string(models.InterfaceRoleStorage) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Must be 'compute' or 'storage'"})
		return
	}

	// Find node by ID or name
	var node models.ComputeNode
	if err := database.DB.Where("id = ? OR name = ?", nodeID, nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	// Check max 2 interfaces per node
	var interfaceCount int64
	database.DB.Model(&models.ComputeNodeInterface{}).Where("compute_node_id = ?", node.ID).Count(&interfaceCount)
	if interfaceCount >= 2 {
		c.JSON(http.StatusConflict, gin.H{"error": "Node already has maximum of 2 interfaces (compute and storage)"})
		return
	}

	// Check if interface with this role already exists (no duplicate roles)
	var existing models.ComputeNodeInterface
	if err := database.DB.Where("compute_node_id = ? AND role = ?", node.ID, input.Role).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Interface with this role already exists for this node"})
		return
	}

	iface := models.ComputeNodeInterface{
		ID:            uuid.New().String(),
		ComputeNodeID: node.ID,
		Role:          models.InterfaceRole(input.Role),
		Hostname:      input.Hostname,
		IPAddress:     input.IPAddress,
		MACAddress:    input.MACAddress,
	}

	if err := database.DB.Create(&iface).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create interface"})
		return
	}

	c.JSON(http.StatusCreated, iface)
}

// UpdateInterfaceInput represents the input for updating an interface
type UpdateInterfaceInput struct {
	Hostname   *string `json:"hostname"`
	IPAddress  *string `json:"ip_address"`
	MACAddress *string `json:"mac_address"`
}

// UpdateInterface updates an interface
func (h *InterfaceHandler) UpdateInterface(c *gin.Context) {
	nodeID := c.Param("id")
	ifaceID := c.Param("interfaceId")

	var input UpdateInterfaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find node by ID or name
	var node models.ComputeNode
	if err := database.DB.Where("id = ? OR name = ?", nodeID, nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	var iface models.ComputeNodeInterface
	if err := database.DB.Where("id = ? AND compute_node_id = ?", ifaceID, node.ID).First(&iface).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Interface not found"})
		return
	}

	updates := make(map[string]interface{})
	if input.Hostname != nil {
		updates["hostname"] = *input.Hostname
	}
	if input.IPAddress != nil {
		updates["ip_address"] = *input.IPAddress
	}
	if input.MACAddress != nil {
		updates["mac_address"] = *input.MACAddress
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&iface).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update interface"})
			return
		}
	}

	database.DB.First(&iface, "id = ?", iface.ID)
	c.JSON(http.StatusOK, iface)
}

// DeleteInterface deletes an interface
func (h *InterfaceHandler) DeleteInterface(c *gin.Context) {
	nodeID := c.Param("id")
	ifaceID := c.Param("interfaceId")

	// Find node by ID or name
	var node models.ComputeNode
	if err := database.DB.Where("id = ? OR name = ?", nodeID, nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	var iface models.ComputeNodeInterface
	if err := database.DB.Where("id = ? AND compute_node_id = ?", ifaceID, node.ID).First(&iface).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Interface not found"})
		return
	}

	// Unlink any port mappings from this interface
	if err := database.DB.Model(&models.ComputeNodePortMapping{}).
		Where("interface_id = ?", iface.ID).
		Update("interface_id", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlink port mappings"})
		return
	}

	if err := database.DB.Delete(&iface).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete interface"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Interface deleted"})
}

// AssignPortMappingInput represents the input for assigning a port mapping to an interface
type AssignPortMappingInput struct {
	InterfaceID *string `json:"interface_id"` // nil to unassign
}

// AssignPortMapping assigns a port mapping to an interface
func (h *InterfaceHandler) AssignPortMapping(c *gin.Context) {
	nodeID := c.Param("id")
	mappingID := c.Param("mappingId")

	var input AssignPortMappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find node by ID or name
	var node models.ComputeNode
	if err := database.DB.Where("id = ? OR name = ?", nodeID, nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	var mapping models.ComputeNodePortMapping
	if err := database.DB.Where("id = ? AND compute_node_id = ?", mappingID, node.ID).First(&mapping).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Port mapping not found"})
		return
	}

	// Track old interface to detect unassignment from storage
	var oldInterfaceID *string
	var wasStorageInterface bool
	if mapping.InterfaceID != nil {
		oldInterfaceID = mapping.InterfaceID
		var oldIface models.ComputeNodeInterface
		if err := database.DB.First(&oldIface, "id = ?", *oldInterfaceID).Error; err == nil {
			wasStorageInterface = oldIface.Role == models.InterfaceRoleStorage
		}
	}

	// Validate interface belongs to same node if provided
	if input.InterfaceID != nil && *input.InterfaceID != "" {
		var iface models.ComputeNodeInterface
		if err := database.DB.Where("id = ? AND compute_node_id = ?", *input.InterfaceID, node.ID).First(&iface).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Interface not found or doesn't belong to this node"})
			return
		}
		mapping.InterfaceID = input.InterfaceID
	} else {
		mapping.InterfaceID = nil
	}

	if err := database.DB.Save(&mapping).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update port mapping"})
		return
	}

	// Update storage SG selectors if assigned to or unassigned from a storage interface
	if h.storageService != nil {
		var isStorageInterface bool
		if input.InterfaceID != nil && *input.InterfaceID != "" {
			var iface models.ComputeNodeInterface
			if err := database.DB.First(&iface, "id = ?", *input.InterfaceID).Error; err == nil {
				isStorageInterface = iface.Role == models.InterfaceRoleStorage
			}
		}
		// Update if assigned to storage OR unassigned from storage
		if isStorageInterface || wasStorageInterface {
			h.updateStorageSGSelectors(node.ID, node.Name)
		}
	}
	_ = oldInterfaceID // silence unused warning

	// Reload with associations
	database.DB.Preload("SwitchPort.Switch").First(&mapping, "id = ?", mapping.ID)
	c.JSON(http.StatusOK, mapping)
}

// updateStorageSGSelectors updates the storage SG selectors in NDFC for a node
func (h *InterfaceHandler) updateStorageSGSelectors(nodeID, nodeName string) {
	if h.storageService == nil {
		return
	}

	ctx := context.Background()

	// Get storage interface for this node
	var storageIface models.ComputeNodeInterface
	if err := database.DB.Where("compute_node_id = ? AND role = ?", nodeID, models.InterfaceRoleStorage).First(&storageIface).Error; err != nil {
		return
	}

	// Get all port mappings for the storage interface
	var mappings []models.ComputeNodePortMapping
	if err := database.DB.Where("interface_id = ?", storageIface.ID).
		Preload("SwitchPort.Switch").
		Find(&mappings).Error; err != nil {
		return
	}

	// Build storage port info
	storagePorts := make([]services.StoragePortInfo, 0, len(mappings))
	for _, m := range mappings {
		if m.SwitchPort != nil && m.SwitchPort.Switch != nil {
			storagePorts = append(storagePorts, services.StoragePortInfo{
				SwitchPortID:  m.SwitchPortID,
				SerialNumber:  m.SwitchPort.Switch.SerialNumber,
				InterfaceName: m.SwitchPort.Name,
				NodeName:      nodeName,
				NodeID:        nodeID,
			})
		}
	}

	// Get the node
	var node models.ComputeNode
	if err := database.DB.First(&node, "id = ?", nodeID).Error; err != nil {
		return
	}

	// Update storage SG with new selectors using the configured storage network
	networkName := h.storageService.GetStorageNetworkName()
	if _, err := h.storageService.EnsureNodeStorageSG(ctx, &node, storagePorts, networkName); err != nil {
		logger.Warn("Failed to update storage SG selectors",
			zap.String("node", nodeName),
			zap.Error(err))
	} else {
		logger.Info("Updated storage SG selectors",
			zap.String("node", nodeName),
			zap.Int("port_count", len(storagePorts)))
	}
}

// BulkPortAssignment represents a single port assignment in a bulk operation
type BulkPortAssignment struct {
	SwitchPortID string  `json:"switch_port_id" binding:"required"`
	NodeID       *string `json:"node_id"`      // nil to unassign
	InterfaceID  *string `json:"interface_id"` // nil for no interface assignment
}

// BulkAssignPortMappingsInput represents the input for bulk port mapping assignment
type BulkAssignPortMappingsInput struct {
	Assignments []BulkPortAssignment `json:"assignments" binding:"required"`
}

// BulkAssignPortMappings handles bulk assignment of switch ports to nodes and interfaces
func (h *InterfaceHandler) BulkAssignPortMappings(c *gin.Context) {
	var input BulkAssignPortMappingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(input.Assignments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No assignments provided"})
		return
	}

	// Track nodes that need storage SG updates
	affectedNodes := make(map[string]bool)
	results := make([]gin.H, 0, len(input.Assignments))

	for _, assignment := range input.Assignments {
		result := gin.H{"switch_port_id": assignment.SwitchPortID}

		// Find existing port mapping by switch_port_id
		var mapping models.ComputeNodePortMapping
		err := database.DB.Where("switch_port_id = ?", assignment.SwitchPortID).First(&mapping).Error

		if assignment.NodeID == nil || *assignment.NodeID == "" {
			// Unassign: delete the mapping if it exists
			if err == nil {
				// Track old node for storage SG update
				if mapping.InterfaceID != nil {
					var oldIface models.ComputeNodeInterface
					if database.DB.First(&oldIface, "id = ?", *mapping.InterfaceID).Error == nil {
						if oldIface.Role == models.InterfaceRoleStorage {
							affectedNodes[mapping.ComputeNodeID] = true
						}
					}
				}
				if err := database.DB.Delete(&mapping).Error; err != nil {
					result["error"] = "Failed to delete mapping"
					result["success"] = false
				} else {
					result["success"] = true
					result["action"] = "deleted"
				}
			} else {
				result["success"] = true
				result["action"] = "no_change"
			}
		} else {
			// Assign to node
			var node models.ComputeNode
			if err := database.DB.Where("id = ? OR name = ?", *assignment.NodeID, *assignment.NodeID).First(&node).Error; err != nil {
				result["error"] = "Node not found"
				result["success"] = false
				results = append(results, result)
				continue
			}

			// Validate interface if provided
			var interfaceID *string
			if assignment.InterfaceID != nil && *assignment.InterfaceID != "" {
				var iface models.ComputeNodeInterface
				if err := database.DB.Where("id = ? AND compute_node_id = ?", *assignment.InterfaceID, node.ID).First(&iface).Error; err != nil {
					result["error"] = "Interface not found or doesn't belong to this node"
					result["success"] = false
					results = append(results, result)
					continue
				}
				// Check if this interface already has a port mapping (excluding current port)
				var existingMapping models.ComputeNodePortMapping
				query := database.DB.Where("interface_id = ? AND switch_port_id != ?", *assignment.InterfaceID, assignment.SwitchPortID)
				if query.First(&existingMapping).Error == nil {
					result["error"] = "Interface already has a port assigned. Each interface can only have one port."
					result["success"] = false
					results = append(results, result)
					continue
				}
				interfaceID = assignment.InterfaceID
				if iface.Role == models.InterfaceRoleStorage {
					affectedNodes[node.ID] = true
				}
			}

			if err == nil {
				// Update existing mapping
				oldNodeID := mapping.ComputeNodeID
				oldInterfaceID := mapping.InterfaceID

				// Track old node if it had a storage interface
				if oldInterfaceID != nil {
					var oldIface models.ComputeNodeInterface
					if database.DB.First(&oldIface, "id = ?", *oldInterfaceID).Error == nil {
						if oldIface.Role == models.InterfaceRoleStorage {
							affectedNodes[oldNodeID] = true
						}
					}
				}

				mapping.ComputeNodeID = node.ID
				mapping.InterfaceID = interfaceID
				if err := database.DB.Save(&mapping).Error; err != nil {
					result["error"] = "Failed to update mapping"
					result["success"] = false
				} else {
					result["success"] = true
					result["action"] = "updated"
					result["mapping_id"] = mapping.ID
				}
			} else {
				// Create new mapping
				mapping = models.ComputeNodePortMapping{
					ID:            uuid.New().String(),
					ComputeNodeID: node.ID,
					SwitchPortID:  assignment.SwitchPortID,
					InterfaceID:   interfaceID,
				}
				if err := database.DB.Create(&mapping).Error; err != nil {
					result["error"] = "Failed to create mapping"
					result["success"] = false
				} else {
					result["success"] = true
					result["action"] = "created"
					result["mapping_id"] = mapping.ID
				}
			}
		}
		results = append(results, result)
	}

	// Update storage SGs for affected nodes
	if h.storageService != nil {
		for nodeID := range affectedNodes {
			var node models.ComputeNode
			if database.DB.First(&node, "id = ?", nodeID).Error == nil {
				h.updateStorageSGSelectors(node.ID, node.Name)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(input.Assignments),
	})
}
