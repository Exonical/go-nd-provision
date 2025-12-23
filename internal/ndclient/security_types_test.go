package ndclient

import (
	"encoding/json"
	"testing"
)

func TestContractAssociation_JSONMarshal_WithIDs(t *testing.T) {
	groupID := 42
	assoc := ContractAssociation{
		VRFName:      "prod-vrf",
		SrcGroupID:   &groupID,
		DstGroupID:   &groupID,
		SrcGroupName: "MyGroup",
		DstGroupName: "MyGroup",
		ContractName: "permit-all",
		Attach:       true,
	}

	data, err := json.Marshal(assoc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Should contain srcGroupId and dstGroupId
	jsonStr := string(data)
	if !contains(jsonStr, `"srcGroupId":42`) {
		t.Errorf("expected srcGroupId:42, got %s", jsonStr)
	}
	if !contains(jsonStr, `"dstGroupId":42`) {
		t.Errorf("expected dstGroupId:42, got %s", jsonStr)
	}
}

func TestContractAssociation_JSONMarshal_NamesOnly(t *testing.T) {
	assoc := ContractAssociation{
		VRFName:      "prod-vrf",
		SrcGroupName: "SourceGroup",
		DstGroupName: "DestGroup",
		ContractName: "permit-all",
		Attach:       true,
	}

	data, err := json.Marshal(assoc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Should NOT contain srcGroupId or dstGroupId (nil pointers omitted)
	jsonStr := string(data)
	if contains(jsonStr, `"srcGroupId"`) {
		t.Errorf("srcGroupId should be omitted when nil, got %s", jsonStr)
	}
	if contains(jsonStr, `"dstGroupId"`) {
		t.Errorf("dstGroupId should be omitted when nil, got %s", jsonStr)
	}
	// Should contain names
	if !contains(jsonStr, `"srcGroupName":"SourceGroup"`) {
		t.Errorf("expected srcGroupName, got %s", jsonStr)
	}
}

func TestContractAssociation_JSONMarshal_AttachAlwaysSent(t *testing.T) {
	// Even when false, attach should be sent (NDFC requires it)
	assoc := ContractAssociation{
		VRFName:      "prod-vrf",
		SrcGroupName: "Src",
		DstGroupName: "Dst",
		ContractName: "contract",
		Attach:       false,
	}

	data, err := json.Marshal(assoc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"attach":false`) {
		t.Errorf("attach:false should be present, got %s", jsonStr)
	}
}

func TestContractAssociation_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"vrfName": "test-vrf",
		"srcGroupId": 100,
		"dstGroupId": 200,
		"srcGroupName": "Src",
		"dstGroupName": "Dst",
		"contractName": "my-contract",
		"attach": true
	}`

	var assoc ContractAssociation
	if err := json.Unmarshal([]byte(jsonStr), &assoc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if assoc.VRFName != "test-vrf" {
		t.Errorf("expected vrfName=test-vrf, got %s", assoc.VRFName)
	}
	if assoc.SrcGroupID == nil || *assoc.SrcGroupID != 100 {
		t.Errorf("expected srcGroupId=100, got %v", assoc.SrcGroupID)
	}
	if assoc.DstGroupID == nil || *assoc.DstGroupID != 200 {
		t.Errorf("expected dstGroupId=200, got %v", assoc.DstGroupID)
	}
	if !assoc.Attach {
		t.Error("expected attach=true")
	}
}

func TestSecurityGroup_JSONMarshal_GroupIDOmitted(t *testing.T) {
	// On create, GroupID should be nil and omitted
	group := SecurityGroup{
		GroupName: "test-group",
		Attach:    true,
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, `"groupId"`) {
		t.Errorf("groupId should be omitted when nil, got %s", jsonStr)
	}
}

func TestSecurityGroup_JSONMarshal_GroupIDPresent(t *testing.T) {
	// On update/response, GroupID should be present
	groupID := 123
	group := SecurityGroup{
		GroupID:   &groupID,
		GroupName: "test-group",
		Attach:    true,
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"groupId":123`) {
		t.Errorf("expected groupId:123, got %s", jsonStr)
	}
}

func TestBatchResponse_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"totalCount": 3,
		"failedCount": 1,
		"successCount": 2,
		"code": "200",
		"message": "Partial success",
		"failureList": [
			{"resourceId": "123", "code": "409", "message": "Conflict"}
		]
	}`

	var resp BatchResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.TotalCount != 3 {
		t.Errorf("expected totalCount=3, got %d", resp.TotalCount)
	}
	if resp.FailedCount != 1 {
		t.Errorf("expected failedCount=1, got %d", resp.FailedCount)
	}
	if len(resp.FailureList) != 1 {
		t.Errorf("expected 1 failure, got %d", len(resp.FailureList))
	}
	if resp.FailureList[0].Code != "409" {
		t.Errorf("expected failure code=409, got %s", resp.FailureList[0].Code)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
