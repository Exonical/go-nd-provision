package handlers

import (
	"fmt"
	"net/http"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
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
func (h *FabricHandler) SyncFabrics(c *gin.Context) {
	fabrics, err := h.ndClient.LANFabric().GetFabricsNDFC(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, f := range fabrics {
		fabric := models.Fabric{
			ID:   f.ID,
			Name: f.Name,
			Type: f.Type,
		}
		database.DB.Save(&fabric)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Fabrics synced", "count": len(fabrics)})
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
func (h *FabricHandler) SyncSwitches(c *gin.Context) {
	fabricName := c.Param("id")

	// Find or create fabric record
	var fabric models.Fabric
	if err := database.DB.Where("name = ?", fabricName).First(&fabric).Error; err != nil {
		// Create fabric if it doesn't exist
		fabric = models.Fabric{
			ID:   uuid.New().String(),
			Name: fabricName,
			Type: "VXLAN",
		}
		database.DB.Create(&fabric)
	}

	switches, err := h.ndClient.LANFabric().GetSwitchesNDFC(c.Request.Context(), fabricName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Only import ToR, Leaf, or Border switches (not spines)
	var imported int
	for _, s := range switches {
		if !lanfabric.IsLeafOrBorder(s.SwitchRole) {
			continue
		}

		sw := models.Switch{
			ID:           fmt.Sprintf("%d", s.SwitchDbID),
			Name:         s.LogicalName,
			SerialNumber: s.SerialNumber,
			Model:        s.Model,
			IPAddress:    s.IPAddress,
			FabricID:     fabric.ID,
		}
		database.DB.Save(&sw)
		imported++
	}

	c.JSON(http.StatusOK, gin.H{"message": "Switches synced", "count": imported, "total": len(switches)})
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
// Uses the switch's serial number to query NDFC interface API
func (h *FabricHandler) SyncSwitchPorts(c *gin.Context) {
	switchID := c.Param("switchId")

	// Get switch to find serial number and fabric
	var sw models.Switch
	if err := database.DB.Preload("Fabric").First(&sw, "id = ?", switchID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Switch not found"})
		return
	}

	// Get uplink ports to exclude (inter-switch links)
	var uplinks map[string]bool
	if sw.Fabric != nil {
		var err error
		uplinks, err = h.ndClient.LANFabric().GetUplinkPortsNDFC(c.Request.Context(), sw.Fabric.Name)
		if err != nil {
			uplinks = make(map[string]bool)
		}
	} else {
		uplinks = make(map[string]bool)
	}

	ports, err := h.ndClient.LANFabric().GetSwitchPortsNDFC(c.Request.Context(), sw.SerialNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var imported int
	for _, p := range ports {
		// Only import Ethernet interfaces (e.g., Ethernet1/1 or Ethernet1/1/4)
		if !lanfabric.IsEthernetPort(p.Name) {
			continue
		}

		// Skip uplink ports (inter-switch links)
		uplinkKey := sw.SerialNumber + ":" + p.Name
		if uplinks[uplinkKey] {
			continue
		}

		// Upsert by switch_id + name to avoid duplicates
		var existing models.SwitchPort
		if err := database.DB.Where("switch_id = ? AND name = ?", switchID, p.Name).First(&existing).Error; err == nil {
			existing.Description = p.Description
			existing.Speed = p.Speed
			existing.Status = p.AdminState
			database.DB.Save(&existing)
		} else {
			port := models.SwitchPort{
				ID:          uuid.New().String(),
				Name:        p.Name,
				Description: p.Description,
				Speed:       p.Speed,
				Status:      p.AdminState,
				SwitchID:    switchID,
			}
			database.DB.Create(&port)
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ports synced", "count": imported, "total": len(ports)})
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
		Status      string `json:"status"`
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
		Status:      input.Status,
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
