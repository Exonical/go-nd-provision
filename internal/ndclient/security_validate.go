package ndclient

import (
	"fmt"
	"strings"
)

// validateSecurityGroup validates required fields on a SecurityGroup before sending to NDFC
func validateSecurityGroup(g SecurityGroup) error {
	if strings.TrimSpace(g.GroupName) == "" {
		return fmt.Errorf("groupName is required")
	}
	for i, s := range g.IPSelectors {
		if strings.TrimSpace(s.Type) == "" {
			return fmt.Errorf("ipSelectors[%d].type is required", i)
		}
		if strings.TrimSpace(s.VRFName) == "" {
			return fmt.Errorf("ipSelectors[%d].vrfName is required", i)
		}
	}
	return nil
}

// validateSecurityContract validates required fields on a SecurityContract before sending to NDFC
func validateSecurityContract(c SecurityContract) error {
	if strings.TrimSpace(c.ContractName) == "" {
		return fmt.Errorf("contractName is required")
	}
	for i, r := range c.Rules {
		if strings.TrimSpace(r.Direction) == "" {
			return fmt.Errorf("rules[%d].direction is required", i)
		}
		if strings.TrimSpace(r.Action) == "" {
			return fmt.Errorf("rules[%d].action is required", i)
		}
	}
	return nil
}

// validateContractAssociation validates required fields on a ContractAssociation before sending to NDFC
func validateContractAssociation(a ContractAssociation) error {
	if strings.TrimSpace(a.VRFName) == "" {
		return fmt.Errorf("vrfName is required")
	}
	if strings.TrimSpace(a.ContractName) == "" {
		return fmt.Errorf("contractName is required")
	}
	if strings.TrimSpace(a.SrcGroupName) == "" && (a.SrcGroupID == nil || *a.SrcGroupID <= 0) {
		return fmt.Errorf("srcGroupName or srcGroupId is required")
	}
	if strings.TrimSpace(a.DstGroupName) == "" && (a.DstGroupID == nil || *a.DstGroupID <= 0) {
		return fmt.Errorf("dstGroupName or dstGroupId is required")
	}
	return nil
}

// validateSecurityProtocol validates required fields on a SecurityProtocol before sending to NDFC
func validateSecurityProtocol(p SecurityProtocol) error {
	if strings.TrimSpace(p.ProtocolName) == "" {
		return fmt.Errorf("protocolName is required")
	}
	if strings.TrimSpace(p.MatchType) == "" {
		return fmt.Errorf("matchType is required")
	}
	return nil
}

// sanitizeGroupForRequest clears fields that should not be in the request payload
func sanitizeGroupForRequest(g SecurityGroup) SecurityGroup {
	g.FabricName = "" // use path fabric, not payload
	return g
}

// sanitizeContractForRequest clears response-only fields before sending to NDFC
func sanitizeContractForRequest(c SecurityContract) SecurityContract {
	// Currently no response-only fields to clear
	return c
}

// sanitizeAssociationForRequest clears fields that should not be in the request payload
func sanitizeAssociationForRequest(a ContractAssociation) ContractAssociation {
	a.FabricName = "" // use path fabric, not payload
	return a
}
