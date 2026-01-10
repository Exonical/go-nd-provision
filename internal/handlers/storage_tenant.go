package handlers

import (
	"net/http"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StorageTenantHandler handles HTTP requests for storage tenant operations
type StorageTenantHandler struct{}

// NewStorageTenantHandler creates a new StorageTenantHandler
func NewStorageTenantHandler() *StorageTenantHandler {
	return &StorageTenantHandler{}
}

// StorageTenantInput represents the input for creating/updating a storage tenant
type StorageTenantInput struct {
	Key                  string `json:"key" binding:"required"`
	Description          string `json:"description"`
	StorageNetworkName   string `json:"storage_network_name" binding:"required"`
	StorageNetworkSGName string `json:"storage_network_sg_name"`
	StorageDstGroupName  string `json:"storage_dst_group_name" binding:"required"`
	StorageContractName  string `json:"storage_contract_name" binding:"required"`
}

// GetStorageTenants returns all storage tenants
func (h *StorageTenantHandler) GetStorageTenants(c *gin.Context) {
	var tenants []models.StorageTenant
	if err := database.DB.Find(&tenants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

// GetStorageTenant returns a storage tenant by key
func (h *StorageTenantHandler) GetStorageTenant(c *gin.Context) {
	key := c.Param("key")

	var tenant models.StorageTenant
	if err := database.DB.Where("key = ?", key).First(&tenant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage tenant not found"})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

// CreateStorageTenant creates a new storage tenant
func (h *StorageTenantHandler) CreateStorageTenant(c *gin.Context) {
	var input StorageTenantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if tenant key already exists
	var existing models.StorageTenant
	if err := database.DB.Where("key = ?", input.Key).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Storage tenant with this key already exists"})
		return
	}

	tenant := models.StorageTenant{
		ID:                   uuid.New().String(),
		Key:                  input.Key,
		Description:          input.Description,
		StorageNetworkName:   input.StorageNetworkName,
		StorageNetworkSGName: input.StorageNetworkSGName,
		StorageDstGroupName:  input.StorageDstGroupName,
		StorageContractName:  input.StorageContractName,
	}

	if err := database.DB.Create(&tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tenant)
}

// UpdateStorageTenant updates an existing storage tenant
func (h *StorageTenantHandler) UpdateStorageTenant(c *gin.Context) {
	key := c.Param("key")

	var tenant models.StorageTenant
	if err := database.DB.Where("key = ?", key).First(&tenant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage tenant not found"})
		return
	}

	var input StorageTenantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If key is being changed, check for conflicts
	if input.Key != key {
		var existing models.StorageTenant
		if err := database.DB.Where("key = ?", input.Key).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Storage tenant with this key already exists"})
			return
		}
	}

	tenant.Key = input.Key
	tenant.Description = input.Description
	tenant.StorageNetworkName = input.StorageNetworkName
	tenant.StorageNetworkSGName = input.StorageNetworkSGName
	tenant.StorageDstGroupName = input.StorageDstGroupName
	tenant.StorageContractName = input.StorageContractName

	if err := database.DB.Save(&tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tenant)
}

// DeleteStorageTenant deletes a storage tenant
func (h *StorageTenantHandler) DeleteStorageTenant(c *gin.Context) {
	key := c.Param("key")

	var tenant models.StorageTenant
	if err := database.DB.Where("key = ?", key).First(&tenant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage tenant not found"})
		return
	}

	// Check if tenant is in use by any active jobs
	var count int64
	if err := database.DB.Model(&models.Job{}).
		Where("tenant_key = ? AND status NOT IN ?", key, []string{"completed", "failed"}).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Storage tenant is in use by active jobs"})
		return
	}

	if err := database.DB.Delete(&tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Storage tenant deleted"})
}
