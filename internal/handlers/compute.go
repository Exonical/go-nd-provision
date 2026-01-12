package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ComputeHandler struct {
	storageService *services.StorageService
}

func NewComputeHandler(storageService *services.StorageService) *ComputeHandler {
	return &ComputeHandler{
		storageService: storageService,
	}
}

// CreateComputeNode creates a new compute node
func (h *ComputeHandler) CreateComputeNode(c *gin.Context) {
	var input struct {
		Name        string `json:"name" binding:"required"`
		Hostname    string `json:"hostname"`
		IPAddress   string `json:"ip_address"`
		MACAddress  string `json:"mac_address"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node := models.ComputeNode{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Hostname:    input.Hostname,
		IPAddress:   input.IPAddress,
		MACAddress:  input.MACAddress,
		Description: input.Description,
	}

	if err := database.DB.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create storage SG in NDFC for this node (best-effort, no port selectors yet)
	if h.storageService != nil {
		ctx := context.Background()
		if _, err := h.storageService.EnsureNodeStorageSG(ctx, &node, nil, ""); err != nil {
			logger.Warn("Failed to create storage SG for new compute node",
				zap.String("node", node.Name),
				zap.Error(err))
		} else {
			logger.Info("Created storage SG for compute node",
				zap.String("node", node.Name))
		}
	}

	c.JSON(http.StatusCreated, node)
}

// GetComputeNodes returns all compute nodes
func (h *ComputeHandler) GetComputeNodes(c *gin.Context) {
	var nodes []models.ComputeNode
	if err := database.DB.Preload("PortMappings").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// findComputeNode resolves a compute node by ID or name
func (h *ComputeHandler) findComputeNode(idOrName string) (*models.ComputeNode, error) {
	var node models.ComputeNode
	// Use Where().First() to avoid GORM logging "record not found" for expected fallback behavior
	if err := database.DB.Where("id = ? OR name = ?", idOrName, idOrName).First(&node).Error; err == nil {
		return &node, nil
	}
	return nil, fmt.Errorf("compute node not found")
}

// GetComputeNode returns a single compute node by ID or name
func (h *ComputeHandler) GetComputeNode(c *gin.Context) {
	idOrName := c.Param("id")
	node, err := h.findComputeNode(idOrName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}
	// Preload relationships
	database.DB.Preload("PortMappings.SwitchPort.Switch").First(node, "id = ?", node.ID)
	c.JSON(http.StatusOK, node)
}

// UpdateComputeNode updates a compute node (by ID or name)
func (h *ComputeHandler) UpdateComputeNode(c *gin.Context) {
	idOrName := c.Param("id")
	node, err := h.findComputeNode(idOrName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	var input struct {
		Name        string `json:"name"`
		Hostname    string `json:"hostname"`
		IPAddress   string `json:"ip_address"`
		MACAddress  string `json:"mac_address"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Name != "" {
		node.Name = input.Name
	}
	if input.Hostname != "" {
		node.Hostname = input.Hostname
	}
	if input.IPAddress != "" {
		node.IPAddress = input.IPAddress
	}
	if input.MACAddress != "" {
		node.MACAddress = input.MACAddress
	}
	if input.Description != "" {
		node.Description = input.Description
	}

	if err := database.DB.Save(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, node)
}

// DeleteComputeNode deletes a compute node (by ID or name)
func (h *ComputeHandler) DeleteComputeNode(c *gin.Context) {
	idOrName := c.Param("id")
	node, err := h.findComputeNode(idOrName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}
	if err := database.DB.Delete(&models.ComputeNode{}, "id = ?", node.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Compute node deleted"})
}

// AddPortMapping maps a compute node to a switch port
// Accepts either:
//   - switch_port_id: full port ID
//   - switch + port_name: simplified lookup (e.g., switch: "site1-leaf1", port_name: "Ethernet1/1")
func (h *ComputeHandler) AddPortMapping(c *gin.Context) {
	nodeIDOrName := c.Param("id")

	var input struct {
		SwitchPortID string `json:"switch_port_id"` // Full port ID (optional if switch + port_name provided)
		Switch       string `json:"switch"`         // Switch name/serial/ID (optional if switch_port_id provided)
		PortName     string `json:"port_name"`      // Port name like "Ethernet1/1" (optional if switch_port_id provided)
		NICName      string `json:"nic_name"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify compute node exists (by ID or name)
	node, err := h.findComputeNode(nodeIDOrName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}

	// Resolve switch port ID
	var port models.SwitchPort
	if input.SwitchPortID != "" {
		// Direct port ID lookup
		if err := database.DB.First(&port, "id = ?", input.SwitchPortID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Switch port not found"})
			return
		}
	} else if input.Switch != "" && input.PortName != "" {
		// Simplified lookup: find switch first, then port by name
		var sw models.Switch
		if err := database.DB.First(&sw, "id = ?", input.Switch).Error; err != nil {
			if err := database.DB.Where("serial_number = ?", input.Switch).First(&sw).Error; err != nil {
				if err := database.DB.Where("name = ?", input.Switch).First(&sw).Error; err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
					return
				}
			}
		}
		// Find port by name within this switch
		if err := database.DB.Where("switch_id = ? AND name = ?", sw.ID, input.PortName).First(&port).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Port not found on switch"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either switch_port_id or (switch + port_name) required"})
		return
	}

	// Delete any existing mapping for this port (reassignment)
	if err := database.DB.Where("switch_port_id = ?", port.ID).Delete(&models.ComputeNodePortMapping{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove existing mapping: " + err.Error()})
		return
	}

	mapping := models.ComputeNodePortMapping{
		ID:            uuid.New().String(),
		ComputeNodeID: node.ID,
		SwitchPortID:  port.ID,
		NICName:       input.NICName,
	}

	if err := database.DB.Create(&mapping).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapping)
}

// GetPortMappings returns all port mappings for a compute node (by ID or name)
func (h *ComputeHandler) GetPortMappings(c *gin.Context) {
	nodeIDOrName := c.Param("id")
	node, err := h.findComputeNode(nodeIDOrName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}
	var mappings []models.ComputeNodePortMapping
	if err := database.DB.Preload("SwitchPort.Switch").Where("compute_node_id = ?", node.ID).Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mappings)
}

// UpdatePortMapping updates a port mapping (NIC name and/or switch port)
func (h *ComputeHandler) UpdatePortMapping(c *gin.Context) {
	mappingID := c.Param("mappingId")

	var mapping models.ComputeNodePortMapping
	if err := database.DB.First(&mapping, "id = ?", mappingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Port mapping not found"})
		return
	}

	var input struct {
		NICName      string `json:"nic_name"`
		SwitchPortID string `json:"switch_port_id"` // Full port ID (optional)
		Switch       string `json:"switch"`         // Switch name/serial/ID (optional)
		PortName     string `json:"port_name"`      // Port name like "Ethernet1/1" (optional)
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.NICName != "" {
		mapping.NICName = input.NICName
	}

	// Update switch port if provided
	if input.SwitchPortID != "" {
		var port models.SwitchPort
		if err := database.DB.First(&port, "id = ?", input.SwitchPortID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Switch port not found"})
			return
		}
		mapping.SwitchPortID = port.ID
	} else if input.Switch != "" && input.PortName != "" {
		// Simplified lookup: find switch first, then port by name
		var sw models.Switch
		if err := database.DB.First(&sw, "id = ?", input.Switch).Error; err != nil {
			if err := database.DB.Where("serial_number = ?", input.Switch).First(&sw).Error; err != nil {
				if err := database.DB.Where("name = ?", input.Switch).First(&sw).Error; err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
					return
				}
			}
		}
		var port models.SwitchPort
		if err := database.DB.Where("switch_id = ? AND name = ?", sw.ID, input.PortName).First(&port).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Port not found on switch"})
			return
		}
		mapping.SwitchPortID = port.ID
	}

	if err := database.DB.Save(&mapping).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload with associations
	database.DB.Preload("SwitchPort.Switch").First(&mapping, "id = ?", mapping.ID)
	c.JSON(http.StatusOK, mapping)
}

// DeletePortMapping removes a port mapping
func (h *ComputeHandler) DeletePortMapping(c *gin.Context) {
	mappingID := c.Param("mappingId")
	if err := database.DB.Delete(&models.ComputeNodePortMapping{}, "id = ?", mappingID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Port mapping deleted"})
}

// GetComputeNodesBySwitch returns all compute nodes connected to a specific switch
func (h *ComputeHandler) GetComputeNodesBySwitch(c *gin.Context) {
	switchIDOrName := c.Param("switchId")

	// Resolve switch by ID, serial number, or name
	var sw models.Switch
	if err := database.DB.First(&sw, "id = ?", switchIDOrName).Error; err != nil {
		if err := database.DB.Where("serial_number = ?", switchIDOrName).First(&sw).Error; err != nil {
			if err := database.DB.Where("name = ?", switchIDOrName).First(&sw).Error; err != nil {
				c.JSON(http.StatusOK, []models.ComputeNodePortMapping{})
				return
			}
		}
	}

	var mappings []models.ComputeNodePortMapping
	if err := database.DB.
		Joins("JOIN switch_ports ON switch_ports.id = compute_node_port_mappings.switch_port_id").
		Where("switch_ports.switch_id = ?", sw.ID).
		Preload("ComputeNode").
		Preload("SwitchPort").
		Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mappings)
}

// GetComputeNodesBySwitchPort returns all compute nodes connected to a specific switch port
func (h *ComputeHandler) GetComputeNodesBySwitchPort(c *gin.Context) {
	portID := c.Param("portId")

	var mappings []models.ComputeNodePortMapping
	if err := database.DB.
		Where("switch_port_id = ?", portID).
		Preload("ComputeNode").
		Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mappings)
}
