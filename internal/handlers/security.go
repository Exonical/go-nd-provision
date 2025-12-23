package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/banglin/go-nd/internal/database"
	"github.com/banglin/go-nd/internal/models"
	"github.com/banglin/go-nd/internal/ndclient"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SecurityHandler struct {
	ndClient *ndclient.Client
	db       *gorm.DB
}

func NewSecurityHandler(client *ndclient.Client) *SecurityHandler {
	return &SecurityHandler{ndClient: client, db: database.DB}
}

// Security Group handlers

type CreateSecurityGroupInput struct {
	GroupName            string                     `json:"group_name" binding:"required"`
	FabricName           string                     `json:"fabric_name" binding:"required"`
	SourceFabric         string                     `json:"source_fabric,omitempty"`
	Attach               bool                       `json:"attach"`
	IPSelectors          []IPSelectorInput          `json:"ip_selectors,omitempty"`
	NetworkSelectors     []NetworkSelectorInput     `json:"network_selectors,omitempty"`
	NetworkPortSelectors []NetworkPortSelectorInput `json:"network_port_selectors,omitempty"`
}

type IPSelectorInput struct {
	Type      string `json:"type"`
	VRFName   string `json:"vrf_name"`
	CreatedBy string `json:"created_by,omitempty"`
}

type NetworkSelectorInput struct {
	VRFName string `json:"vrf_name"`
	Network string `json:"network"`
}

type NetworkPortSelectorInput struct {
	Network       string `json:"network"`
	SwitchID      string `json:"switch_id"`
	InterfaceName string `json:"interface_name"`
}

func (h *SecurityHandler) CreateSecurityGroup(c *gin.Context) {
	var input CreateSecurityGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build IP selectors for ND API
	var ipSelectors []ndclient.IPSelector
	for _, sel := range input.IPSelectors {
		ipSelectors = append(ipSelectors, ndclient.IPSelector{
			Type:       sel.Type,
			VRFName:    sel.VRFName,
			CreatedBy:  sel.CreatedBy,
			DeletedStr: "-",
		})
	}

	// Build network selectors
	var networkSelectors []ndclient.NetworkSelector
	for _, sel := range input.NetworkSelectors {
		networkSelectors = append(networkSelectors, ndclient.NetworkSelector{
			VRFName: sel.VRFName,
			Network: sel.Network,
		})
	}

	// Build network port selectors
	var networkPortSelectors []ndclient.NetworkPortSelector
	for _, sel := range input.NetworkPortSelectors {
		networkPortSelectors = append(networkPortSelectors, ndclient.NetworkPortSelector{
			Network:       sel.Network,
			SwitchID:      sel.SwitchID,
			InterfaceName: sel.InterfaceName,
		})
	}

	// Create in Nexus Dashboard
	ndGroup := &ndclient.SecurityGroup{
		FabricName:           input.FabricName,
		SourceFabric:         input.SourceFabric,
		GroupName:            input.GroupName,
		Attach:               input.Attach,
		IPSelectors:          ipSelectors,
		NetworkSelectors:     networkSelectors,
		NetworkPortSelectors: networkPortSelectors,
	}

	ndResp, err := h.ndClient.CreateSecurityGroup(c.Request.Context(), input.FabricName, ndGroup)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to local database
	group := models.SecurityGroup{
		ID:         uuid.New().String(),
		Name:       input.GroupName,
		NDObjectID: fmt.Sprintf("%d", ndResp.GroupID),
		FabricName: input.FabricName,
	}

	if err := h.db.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"local":  group,
		"remote": ndResp,
	})
}

func (h *SecurityHandler) GetSecurityGroups(c *gin.Context) {
	fabricName := c.Query("fabric_name")

	// If fabric name provided, fetch from NDFC
	if fabricName != "" && h.ndClient != nil {
		groups, err := h.ndClient.GetSecurityGroups(c.Request.Context(), fabricName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, groups)
		return
	}

	// Otherwise return from local database
	var groups []models.SecurityGroup
	if err := h.db.Preload("Selectors.SwitchPort").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *SecurityHandler) GetSecurityGroup(c *gin.Context) {
	id := c.Param("id")
	fabricName := c.Query("fabric_name")

	// If fabric name provided, fetch from NDFC by name
	if fabricName != "" && h.ndClient != nil {
		group, err := h.ndClient.GetSecurityGroupByName(c.Request.Context(), fabricName, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, group)
		return
	}

	// Otherwise return from local database
	var group models.SecurityGroup
	if err := h.db.Preload("Selectors.SwitchPort.Switch").First(&group, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security group not found"})
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *SecurityHandler) DeleteSecurityGroup(c *gin.Context) {
	id := c.Param("id")

	var group models.SecurityGroup
	if err := h.db.First(&group, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security group not found"})
		return
	}

	// Delete from Nexus Dashboard
	if h.ndClient != nil && group.NDObjectID != "" {
		groupID, err := strconv.Atoi(group.NDObjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid NDObjectID: " + group.NDObjectID})
			return
		}
		if groupID > 0 {
			if err := h.ndClient.DeleteSecurityGroup(c.Request.Context(), group.FabricName, groupID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	// Delete selectors and group in transaction
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("security_group_id = ?", id).Delete(&models.PortSelector{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&group).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Security group deleted"})
}

// Security Contract handlers

type CreateSecurityContractInput struct {
	ContractName string              `json:"contract_name" binding:"required"`
	FabricName   string              `json:"fabric_name" binding:"required"`
	Rules        []ContractRuleInput `json:"rules"`
}

type ContractRuleInput struct {
	Direction    string `json:"direction" binding:"required"`
	Action       string `json:"action" binding:"required"`
	ProtocolName string `json:"protocol_name"`
}

func (h *SecurityHandler) CreateSecurityContract(c *gin.Context) {
	var input CreateSecurityContractInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build rules for ND API
	var ndRules []ndclient.ContractRule
	for _, r := range input.Rules {
		ndRules = append(ndRules, ndclient.ContractRule{
			Direction:    r.Direction,
			Action:       r.Action,
			ProtocolName: r.ProtocolName,
		})
	}

	// Create in Nexus Dashboard
	ndReq := &ndclient.SecurityContract{
		ContractName: input.ContractName,
		Rules:        ndRules,
	}

	ndResp, err := h.ndClient.CreateSecurityContract(c.Request.Context(), input.FabricName, ndReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to local database with transaction
	contract := models.SecurityContract{
		ID:         uuid.New().String(),
		Name:       input.ContractName,
		NDObjectID: ndResp.ContractName,
		FabricName: input.FabricName,
	}

	rules := make([]models.ContractRule, 0, len(input.Rules))
	for _, r := range input.Rules {
		rules = append(rules, models.ContractRule{
			ID:                 uuid.New().String(),
			SecurityContractID: contract.ID,
			Name:               r.ProtocolName,
			Action:             r.Action,
		})
	}

	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&contract).Error; err != nil {
			return err
		}
		if len(rules) > 0 {
			if err := tx.Create(&rules).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"local":  contract,
		"remote": ndResp,
	})
}

func (h *SecurityHandler) GetSecurityContracts(c *gin.Context) {
	var contracts []models.SecurityContract
	if err := h.db.Preload("Rules").Find(&contracts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contracts)
}

func (h *SecurityHandler) GetSecurityContract(c *gin.Context) {
	id := c.Param("id")
	var contract models.SecurityContract
	if err := h.db.Preload("Rules").First(&contract, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security contract not found"})
		return
	}
	c.JSON(http.StatusOK, contract)
}

func (h *SecurityHandler) DeleteSecurityContract(c *gin.Context) {
	id := c.Param("id")

	var contract models.SecurityContract
	if err := h.db.First(&contract, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security contract not found"})
		return
	}

	// Delete from Nexus Dashboard using stored FabricName
	if h.ndClient != nil && contract.Name != "" && contract.FabricName != "" {
		if err := h.ndClient.DeleteSecurityContract(c.Request.Context(), contract.FabricName, contract.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Delete rules and contract in transaction
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("security_contract_id = ?", id).Delete(&models.ContractRule{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&contract).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Security contract deleted"})
}

// Security Association (Contract Association) handlers

type CreateSecurityAssociationInput struct {
	FabricName   string `json:"fabric_name" binding:"required"`
	VRFName      string `json:"vrf_name" binding:"required"`
	SrcGroupID   int    `json:"src_group_id" binding:"required"`
	DstGroupID   int    `json:"dst_group_id" binding:"required"`
	SrcGroupName string `json:"src_group_name" binding:"required"`
	DstGroupName string `json:"dst_group_name" binding:"required"`
	ContractName string `json:"contract_name" binding:"required"`
	Attach       bool   `json:"attach"`
}

func (h *SecurityHandler) CreateSecurityAssociation(c *gin.Context) {
	var input CreateSecurityAssociationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in Nexus Dashboard
	ndReq := &ndclient.ContractAssociation{
		FabricName:   input.FabricName,
		VRFName:      input.VRFName,
		SrcGroupName: input.SrcGroupName,
		DstGroupName: input.DstGroupName,
		ContractName: input.ContractName,
		Attach:       input.Attach,
	}

	ndResp, err := h.ndClient.CreateSecurityAssociation(c.Request.Context(), input.FabricName, ndReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to local database with all fields needed for remote deletion
	association := models.SecurityAssociation{
		ID:           uuid.New().String(),
		Name:         fmt.Sprintf("%s-%s-%s", input.SrcGroupName, input.DstGroupName, input.ContractName),
		FabricName:   input.FabricName,
		VRFName:      input.VRFName,
		ContractName: input.ContractName,
		SrcGroupNDID: input.SrcGroupID,
		DstGroupNDID: input.DstGroupID,
	}

	if err := h.db.Create(&association).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"local":  association,
		"remote": ndResp,
	})
}

func (h *SecurityHandler) GetSecurityAssociations(c *gin.Context) {
	var associations []models.SecurityAssociation
	if err := h.db.
		Preload("ProviderGroup").
		Preload("ConsumerGroup").
		Preload("SecurityContract").
		Find(&associations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, associations)
}

func (h *SecurityHandler) GetSecurityAssociation(c *gin.Context) {
	id := c.Param("id")
	var association models.SecurityAssociation
	if err := h.db.
		Preload("ProviderGroup.Selectors.SwitchPort").
		Preload("ConsumerGroup.Selectors.SwitchPort").
		Preload("SecurityContract.Rules").
		First(&association, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security association not found"})
		return
	}
	c.JSON(http.StatusOK, association)
}

func (h *SecurityHandler) DeleteSecurityAssociation(c *gin.Context) {
	id := c.Param("id")

	var association models.SecurityAssociation
	if err := h.db.First(&association, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security association not found"})
		return
	}

	// Delete from Nexus Dashboard using stored fields
	if h.ndClient != nil && association.FabricName != "" && association.SrcGroupNDID > 0 && association.DstGroupNDID > 0 {
		if err := h.ndClient.DeleteSecurityAssociation(c.Request.Context(), association.FabricName, association.VRFName, association.SrcGroupNDID, association.DstGroupNDID, association.ContractName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Delete from local database
	if err := h.db.Delete(&association).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Security association deleted"})
}

// ListNDFCSecurityGroups lists all security groups from NDFC
func (h *SecurityHandler) ListNDFCSecurityGroups(c *gin.Context) {
	fabricName := c.Query("fabric")
	if fabricName == "" {
		fabricName = "DevNet_VxLAN_Fabric"
	}

	groups, err := h.ndClient.GetSecurityGroups(c.Request.Context(), fabricName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// DeleteNDFCSecurityGroup deletes a security group directly from NDFC by group ID
func (h *SecurityHandler) DeleteNDFCSecurityGroup(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	fabricName := c.Query("fabric")
	if fabricName == "" {
		fabricName = "DevNet_VxLAN_Fabric"
	}

	if err := h.ndClient.DeleteSecurityGroup(c.Request.Context(), fabricName, groupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Security group deleted from NDFC", "groupId": groupID})
}
