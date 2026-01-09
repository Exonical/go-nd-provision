package handlers

import (
	"fmt"
	"net/http"

	"github.com/banglin/go-nd/internal/cache"
	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/sync"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FabricHandler struct {
	ndClient *ndclient.Client
}

func NewFabricHandler(client *ndclient.Client) *FabricHandler {
	return &FabricHandler{ndClient: client}
}

// SyncFabrics syncs fabrics from Nexus Dashboard to local database
// Uses the shared sync.SyncFabrics helper for consistent upsert behavior
func (h *FabricHandler) SyncFabrics(c *gin.Context) {
	result, err := sync.SyncFabrics(c.Request.Context(), database.DB, h.ndClient.LANFabric())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Fabrics synced", "count": result.Synced})
}

// CreateFabric creates a fabric record manually (for testing/setup)
func (h *FabricHandler) CreateFabric(c *gin.Context) {
	var input struct {
		ID   string `json:"id"`
		Name string `json:"name" binding:"required"`
		Type string `json:"type"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fabric := models.Fabric{
		ID:   input.ID,
		Name: input.Name,
		Type: input.Type,
	}
	if fabric.ID == "" {
		fabric.ID = uuid.New().String()
	}

	if err := database.DB.Create(&fabric).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, fabric)
}

// GetFabrics returns all fabrics from local database
func (h *FabricHandler) GetFabrics(c *gin.Context) {
	var fabrics []models.Fabric
	if err := database.DB.Find(&fabrics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, fabrics)
}

// GetFabric returns a single fabric by ID
func (h *FabricHandler) GetFabric(c *gin.Context) {
	id := c.Param("id")
	var fabric models.Fabric
	if err := database.DB.Preload("Switches").First(&fabric, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
		return
	}
	c.JSON(http.StatusOK, fabric)
}

// SyncSwitches syncs switches for a fabric from Nexus Dashboard
// Uses the shared sync.SyncFabricSwitches helper for consistent upsert behavior
func (h *FabricHandler) SyncSwitches(c *gin.Context) {
	fabricIDOrName := c.Param("id")

	// Find fabric by ID first, then by name
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	result, err := sync.SyncFabricSwitches(c.Request.Context(), database.DB, h.ndClient.LANFabric(), &fabric)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Switches synced", "count": result.Synced, "total": result.Total})
}

// CreateSwitch creates a switch record manually (for testing/setup)
func (h *FabricHandler) CreateSwitch(c *gin.Context) {
	fabricID := c.Param("id")

	var input struct {
		ID           string `json:"id"`
		Name         string `json:"name" binding:"required"`
		SerialNumber string `json:"serial_number" binding:"required"`
		Model        string `json:"model"`
		IPAddress    string `json:"ip_address"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sw := models.Switch{
		ID:           input.ID,
		Name:         input.Name,
		SerialNumber: input.SerialNumber,
		Model:        input.Model,
		IPAddress:    input.IPAddress,
		FabricID:     fabricID,
	}
	if sw.ID == "" {
		sw.ID = uuid.New().String()
	}

	if err := database.DB.Create(&sw).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sw)
}

// GetSwitches returns all switches for a fabric
func (h *FabricHandler) GetSwitches(c *gin.Context) {
	fabricIDOrName := c.Param("id")

	// Find fabric by ID first, then by name (consistent with SyncSwitches)
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	var switches []models.Switch
	if err := database.DB.Where("fabric_id = ?", fabric.ID).Find(&switches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, switches)
}

// findSwitch resolves a switch by ID, serial number, or name within a fabric
func (h *FabricHandler) findSwitch(fabricID, switchIDOrSerial string) (*models.Switch, error) {
	var sw models.Switch
	// Try by ID first
	if err := database.DB.Where("id = ? AND fabric_id = ?", switchIDOrSerial, fabricID).First(&sw).Error; err == nil {
		return &sw, nil
	}
	// Try by serial number
	if err := database.DB.Where("serial_number = ? AND fabric_id = ?", switchIDOrSerial, fabricID).First(&sw).Error; err == nil {
		return &sw, nil
	}
	// Try by name
	if err := database.DB.Where("name = ? AND fabric_id = ?", switchIDOrSerial, fabricID).First(&sw).Error; err == nil {
		return &sw, nil
	}
	return nil, fmt.Errorf("switch not found")
}

// GetSwitch returns a single switch by ID, serial number, or name
func (h *FabricHandler) GetSwitch(c *gin.Context) {
	fabricIDOrName := c.Param("id")
	switchIDOrSerial := c.Param("switchId")

	// Find fabric first
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	sw, err := h.findSwitch(fabric.ID, switchIDOrSerial)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	// Preload ports
	database.DB.Preload("Ports").First(sw, "id = ?", sw.ID)
	c.JSON(http.StatusOK, sw)
}

// SyncAllPorts syncs ports for ALL switches in a fabric from Nexus Dashboard
func (h *FabricHandler) SyncAllPorts(c *gin.Context) {
	fabricIDOrName := c.Param("id")

	// Find fabric by ID first, then by name
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	// Get all switches for the fabric
	var switches []models.Switch
	if err := database.DB.Where("fabric_id = ?", fabric.ID).Find(&switches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(switches) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No switches found in fabric", "switches": 0, "ports": 0})
		return
	}

	// Get uplink ports to exclude (inter-switch links) - uses cache if available
	uplinks := sync.GetUplinksWithCache(c.Request.Context(), h.ndClient.LANFabric(), fabric.Name, cache.Client)

	var totalPorts int
	var totalErrors int
	var switchResults []gin.H

	for _, sw := range switches {
		if sw.SerialNumber == "" {
			continue
		}

		result, err := sync.SyncSwitchPorts(
			c.Request.Context(),
			database.DB,
			h.ndClient.LANFabric(),
			sw.ID,
			sw.SerialNumber,
			uplinks,
		)
		if err != nil {
			totalErrors++
			switchResults = append(switchResults, gin.H{
				"switch": sw.Name,
				"error":  err.Error(),
			})
			continue
		}

		totalPorts += result.Synced
		switchResults = append(switchResults, gin.H{
			"switch": sw.Name,
			"synced": result.Synced,
			"total":  result.Total,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Ports synced for all switches",
		"switches": len(switches),
		"ports":    totalPorts,
		"errors":   totalErrors,
		"details":  switchResults,
	})
}

// SyncSwitchPorts syncs ports for a switch from Nexus Dashboard (by ID, serial, or name)
// Uses the shared sync.SyncSwitchPorts helper (same as background worker)
func (h *FabricHandler) SyncSwitchPorts(c *gin.Context) {
	fabricIDOrName := c.Param("id")
	switchIDOrSerial := c.Param("switchId")

	// Find fabric first
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	sw, err := h.findSwitch(fabric.ID, switchIDOrSerial)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	// Get uplink ports to exclude (inter-switch links) - uses cache if available
	uplinks := sync.GetUplinksWithCache(c.Request.Context(), h.ndClient.LANFabric(), fabric.Name, cache.Client)

	// Use shared helper for port sync
	result, err := sync.SyncSwitchPorts(
		c.Request.Context(),
		database.DB,
		h.ndClient.LANFabric(),
		sw.ID,
		sw.SerialNumber,
		uplinks,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to sync ports: %v", err)})
		return
	}

	if result.Synced == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No Ethernet ports found", "count": 0, "total": result.Total})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ports synced", "count": result.Synced, "total": result.Total})
}

// DeleteSwitchPorts deletes all ports for a switch (by ID, serial, or name)
func (h *FabricHandler) DeleteSwitchPorts(c *gin.Context) {
	fabricIDOrName := c.Param("id")
	switchIDOrSerial := c.Param("switchId")

	// Find fabric first
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	sw, err := h.findSwitch(fabric.ID, switchIDOrSerial)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	result := database.DB.Where("switch_id = ?", sw.ID).Delete(&models.SwitchPort{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Ports deleted", "count": result.RowsAffected})
}

// GetSwitchPorts returns all ports for a switch (by ID, serial, or name)
func (h *FabricHandler) GetSwitchPorts(c *gin.Context) {
	fabricIDOrName := c.Param("id")
	switchIDOrSerial := c.Param("switchId")

	// Find fabric first
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	sw, err := h.findSwitch(fabric.ID, switchIDOrSerial)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	var ports []models.SwitchPort
	if err := database.DB.Where("switch_id = ?", sw.ID).Find(&ports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ports)
}

// GetSwitchPort returns a single port by ID
func (h *FabricHandler) GetSwitchPort(c *gin.Context) {
	portID := c.Param("portId")
	var port models.SwitchPort
	if err := database.DB.First(&port, "id = ?", portID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Port not found"})
		return
	}
	c.JSON(http.StatusOK, port)
}

// CreateSwitchPort creates a new switch port manually (switch by ID, serial, or name)
func (h *FabricHandler) CreateSwitchPort(c *gin.Context) {
	fabricIDOrName := c.Param("id")
	switchIDOrSerial := c.Param("switchId")

	// Find fabric first
	var fabric models.Fabric
	if err := database.DB.First(&fabric, "id = ?", fabricIDOrName).Error; err != nil {
		if err := database.DB.Where("name = ?", fabricIDOrName).First(&fabric).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fabric not found"})
			return
		}
	}

	sw, err := h.findSwitch(fabric.ID, switchIDOrSerial)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	var input struct {
		Name        string `json:"name" binding:"required"`
		PortNumber  string `json:"port_number"`
		Description string `json:"description"`
		AdminState  string `json:"admin_state"`
		Speed       string `json:"speed"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	port := models.SwitchPort{
		ID:          uuid.New().String(),
		Name:        input.Name,
		PortNumber:  input.PortNumber,
		Description: input.Description,
		AdminState:  input.AdminState,
		IsPresent:   true,
		Speed:       input.Speed,
		SwitchID:    sw.ID,
	}

	if err := database.DB.Create(&port).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, port)
}

// GetNetworks returns all networks for a fabric from NDFC
func (h *FabricHandler) GetNetworks(c *gin.Context) {
	fabricName := c.Param("id")
	if fabricName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fabric name required"})
		return
	}

	networks, err := h.ndClient.LANFabric().GetNetworksNDFC(c.Request.Context(), fabricName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, networks)
}
