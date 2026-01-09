package handlers

import (
	"net/http"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ComputeHandler struct{}

func NewComputeHandler() *ComputeHandler {
	return &ComputeHandler{}
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

// GetComputeNode returns a single compute node by ID
func (h *ComputeHandler) GetComputeNode(c *gin.Context) {
	id := c.Param("id")
	var node models.ComputeNode
	if err := database.DB.Preload("PortMappings.SwitchPort.Switch").First(&node, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compute node not found"})
		return
	}
	c.JSON(http.StatusOK, node)
}

// UpdateComputeNode updates a compute node
func (h *ComputeHandler) UpdateComputeNode(c *gin.Context) {
	id := c.Param("id")
	var node models.ComputeNode
	if err := database.DB.First(&node, "id = ?", id).Error; err != nil {
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

// DeleteComputeNode deletes a compute node
func (h *ComputeHandler) DeleteComputeNode(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.ComputeNode{}, "id = ?", id).Error; err != nil {
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
	nodeID := c.Param("id")

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

	// Verify compute node exists
	var node models.ComputeNode
	if err := database.DB.First(&node, "id = ?", nodeID).Error; err != nil {
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

	mapping := models.ComputeNodePortMapping{
		ID:            uuid.New().String(),
		ComputeNodeID: nodeID,
		SwitchPortID:  port.ID,
		NICName:       input.NICName,
	}

	if err := database.DB.Create(&mapping).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapping)
}

// GetPortMappings returns all port mappings for a compute node
func (h *ComputeHandler) GetPortMappings(c *gin.Context) {
	nodeID := c.Param("id")
	var mappings []models.ComputeNodePortMapping
	if err := database.DB.Preload("SwitchPort.Switch").Where("compute_node_id = ?", nodeID).Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mappings)
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
	switchID := c.Param("switchId")

	var mappings []models.ComputeNodePortMapping
	if err := database.DB.
		Joins("JOIN switch_ports ON switch_ports.id = compute_node_port_mappings.switch_port_id").
		Where("switch_ports.switch_id = ?", switchID).
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
