package ndclient

import (
	"testing"
)

func TestValidateContractAssociation_Valid_WithIDs(t *testing.T) {
	srcID := 1
	dstID := 2
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		SrcGroupID:   &srcID,
		DstGroupID:   &dstID,
		ContractName: "test-contract",
	}

	if err := validateContractAssociation(assoc); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateContractAssociation_Valid_WithNames(t *testing.T) {
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		SrcGroupName: "src-group",
		DstGroupName: "dst-group",
		ContractName: "test-contract",
	}

	if err := validateContractAssociation(assoc); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateContractAssociation_Valid_Mixed(t *testing.T) {
	srcID := 1
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		SrcGroupID:   &srcID,
		DstGroupName: "dst-group",
		ContractName: "test-contract",
	}

	if err := validateContractAssociation(assoc); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateContractAssociation_MissingVRF(t *testing.T) {
	srcID := 1
	dstID := 2
	assoc := ContractAssociation{
		SrcGroupID:   &srcID,
		DstGroupID:   &dstID,
		ContractName: "test-contract",
	}

	err := validateContractAssociation(assoc)
	if err == nil {
		t.Error("expected error for missing vrfName")
	}
}

func TestValidateContractAssociation_MissingContract(t *testing.T) {
	srcID := 1
	dstID := 2
	assoc := ContractAssociation{
		VRFName:    "test-vrf",
		SrcGroupID: &srcID,
		DstGroupID: &dstID,
	}

	err := validateContractAssociation(assoc)
	if err == nil {
		t.Error("expected error for missing contractName")
	}
}

func TestValidateContractAssociation_MissingSrc(t *testing.T) {
	dstID := 2
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		DstGroupID:   &dstID,
		ContractName: "test-contract",
	}

	err := validateContractAssociation(assoc)
	if err == nil {
		t.Error("expected error for missing src group")
	}
}

func TestValidateContractAssociation_MissingDst(t *testing.T) {
	srcID := 1
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		SrcGroupID:   &srcID,
		ContractName: "test-contract",
	}

	err := validateContractAssociation(assoc)
	if err == nil {
		t.Error("expected error for missing dst group")
	}
}

func TestValidateContractAssociation_ZeroID_Invalid(t *testing.T) {
	zero := 0
	assoc := ContractAssociation{
		VRFName:      "test-vrf",
		SrcGroupID:   &zero, // 0 is not valid
		DstGroupName: "dst-group",
		ContractName: "test-contract",
	}

	err := validateContractAssociation(assoc)
	if err == nil {
		t.Error("expected error for srcGroupId=0")
	}
}

func TestValidateSecurityGroup_Valid(t *testing.T) {
	group := SecurityGroup{
		GroupName: "test-group",
	}

	if err := validateSecurityGroup(group); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateSecurityGroup_MissingName(t *testing.T) {
	group := SecurityGroup{}

	err := validateSecurityGroup(group)
	if err == nil {
		t.Error("expected error for missing groupName")
	}
}

func TestValidateSecurityProtocol_Valid(t *testing.T) {
	proto := SecurityProtocol{
		ProtocolName: "test-protocol",
		MatchType:    "any",
	}

	if err := validateSecurityProtocol(proto); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateSecurityProtocol_MissingName(t *testing.T) {
	proto := SecurityProtocol{
		MatchType: "any",
	}

	err := validateSecurityProtocol(proto)
	if err == nil {
		t.Error("expected error for missing protocolName")
	}
}

func TestValidateSecurityProtocol_MissingMatchType(t *testing.T) {
	proto := SecurityProtocol{
		ProtocolName: "test-protocol",
	}

	err := validateSecurityProtocol(proto)
	if err == nil {
		t.Error("expected error for missing matchType")
	}
}

func TestSanitizeGroupForRequest(t *testing.T) {
	group := SecurityGroup{
		FabricName: "should-be-cleared",
		GroupName:  "test-group",
		Attach:     true,
	}

	sanitized := sanitizeGroupForRequest(group)

	if sanitized.FabricName != "" {
		t.Errorf("expected fabricName to be cleared, got %s", sanitized.FabricName)
	}
	if sanitized.GroupName != "test-group" {
		t.Errorf("expected groupName preserved, got %s", sanitized.GroupName)
	}
}

func TestSanitizeAssociationForRequest(t *testing.T) {
	srcID := 1
	assoc := ContractAssociation{
		FabricName:   "should-be-cleared",
		VRFName:      "test-vrf",
		SrcGroupID:   &srcID,
		ContractName: "test-contract",
	}

	sanitized := sanitizeAssociationForRequest(assoc)

	if sanitized.FabricName != "" {
		t.Errorf("expected fabricName to be cleared, got %s", sanitized.FabricName)
	}
	if sanitized.VRFName != "test-vrf" {
		t.Errorf("expected vrfName preserved, got %s", sanitized.VRFName)
	}
}
