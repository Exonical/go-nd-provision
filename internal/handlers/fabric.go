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
	fabricID := c.Param("id")
	var switches []models.Switch
	if err := database.DB.Where("fabric_id = ?", fabricID).Find(&switches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, switches)
}

// GetSwitch returns a single switch by ID
func (h *FabricHandler) GetSwitch(c *gin.Context) {
	switchID := c.Param("switchId")
	var sw models.Switch
	if err := database.DB.Preload("Ports").First(&sw, "id = ?", switchID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}
	c.JSON(http.StatusOK, sw)
}

// SyncSwitchPorts syncs ports for a switch from Nexus Dashboard
// Uses the shared sync.SyncSwitchPorts helper (same as background worker)
func (h *FabricHandler) SyncSwitchPorts(c *gin.Context) {
	switchID := c.Param("switchId")

	// Get switch to find serial number and fabric
	var sw models.Switch
	if err := database.DB.Preload("Fabric").First(&sw, "id = ?", switchID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	// Get uplink ports to exclude (inter-switch links) - uses cache if available
	var uplinks map[string]bool
	if sw.Fabric != nil {
		uplinks = sync.GetUplinksWithCache(c.Request.Context(), h.ndClient.LANFabric(), sw.Fabric.Name, cache.Client)
	} else {
		uplinks = make(map[string]bool)
	}

	// Use shared helper for port sync
	result, err := sync.SyncSwitchPorts(
		c.Request.Context(),
		database.DB,
		h.ndClient.LANFabric(),
		switchID,
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

// DeleteSwitchPorts deletes all ports for a switch (for cleanup/re-sync)
func (h *FabricHandler) DeleteSwitchPorts(c *gin.Context) {
	switchID := c.Param("switchId")
	result := database.DB.Where("switch_id = ?", switchID).Delete(&models.SwitchPort{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Ports deleted", "count": result.RowsAffected})
}

// GetSwitchPorts returns all ports for a switch
func (h *FabricHandler) GetSwitchPorts(c *gin.Context) {
	switchID := c.Param("switchId")
	var ports []models.SwitchPort
	if err := database.DB.Where("switch_id = ?", switchID).Find(&ports).Error; err != nil {
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

// CreateSwitchPort creates a new switch port manually
func (h *FabricHandler) CreateSwitchPort(c *gin.Context) {
	switchID := c.Param("switchId")

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
		SwitchID:    switchID,
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
