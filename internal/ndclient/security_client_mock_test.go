package ndclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/banglin/go-nd/internal/config"
)

// newTestClient creates a client pointing to a test server
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)

	cfg := &config.NexusDashboardConfig{
		BaseURL:  server.URL,
		APIKey:   "test-api-key",
		Username: "admin",
		Insecure: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	return client, server
}

// TestCreateSecurityGroups_Success tests successful security group creation
func TestCreateSecurityGroups_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/security/fabrics/test-fabric/groups") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := BatchResponseGroups{
			BatchResponse: BatchResponse{
				TotalCount:   1,
				SuccessCount: 1,
				FailedCount:  0,
			},
			SuccessList: []SecurityGroup{
				{GroupName: "test-group", GroupID: intPtr(12345)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	groups, err := client.CreateSecurityGroups(context.Background(), "test-fabric", []SecurityGroup{
		{GroupName: "test-group", GroupID: intPtr(12345)},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].GroupName != "test-group" {
		t.Errorf("expected group name 'test-group', got '%s'", groups[0].GroupName)
	}
}

// TestCreateSecurityGroups_BatchError tests batch error handling
func TestCreateSecurityGroups_BatchError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BatchResponseGroups{
			BatchResponse: BatchResponse{
				TotalCount:   1,
				SuccessCount: 0,
				FailedCount:  1,
				FailureList: []BatchItem{
					{Name: "bad-group", Code: "INVALID", Message: "group name too long"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	_, err := client.CreateSecurityGroups(context.Background(), "test-fabric", []SecurityGroup{
		{GroupName: "bad-group", GroupID: intPtr(12345)},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var batchErr *BatchError
	if !errors.As(err, &batchErr) {
		t.Fatalf("expected BatchError, got %T: %v", err, err)
	}
	if batchErr.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", batchErr.Failed)
	}
}

// TestCreateSecurityGroups_HTTPError tests HTTP error handling
func TestCreateSecurityGroups_HTTPError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	_, err := client.CreateSecurityGroups(context.Background(), "test-fabric", []SecurityGroup{
		{GroupName: "test-group", GroupID: intPtr(12345)},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
}

// TestGetSecurityGroups_Success tests successful security group retrieval
func TestGetSecurityGroups_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}

		groups := []SecurityGroup{
			{GroupName: "group1", GroupID: intPtr(100)},
			{GroupName: "group2", GroupID: intPtr(200)},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(groups)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	groups, err := client.GetSecurityGroups(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

// TestGetSecurityGroupByName_Found tests finding a group by name
func TestGetSecurityGroupByName_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groups := []SecurityGroup{
			{GroupName: "group1", GroupID: intPtr(100)},
			{GroupName: "target-group", GroupID: intPtr(200)},
			{GroupName: "group3", GroupID: intPtr(300)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	group, err := client.GetSecurityGroupByName(context.Background(), "test-fabric", "target-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.GroupName != "target-group" {
		t.Errorf("expected 'target-group', got '%s'", group.GroupName)
	}
	if *group.GroupID != 200 {
		t.Errorf("expected ID 200, got %d", *group.GroupID)
	}
}

// TestGetSecurityGroupByName_NotFound tests group not found error
func TestGetSecurityGroupByName_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groups := []SecurityGroup{
			{GroupName: "group1", GroupID: intPtr(100)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	_, err := client.GetSecurityGroupByName(context.Background(), "test-fabric", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestConfigDeploy_Success tests successful config deploy
func TestConfigDeploy_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/config-deploy") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.ConfigDeploy(context.Background(), "test-fabric", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConfigDeploy_RetryOnInProgress tests retry logic when deploy is in progress
func TestConfigDeploy_RetryOnInProgress(t *testing.T) {
	var attempts int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			// First 2 attempts fail with "deploy in progress"
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Deploy is already in progress"}`))
			return
		}
		// Third attempt succeeds
		w.WriteHeader(http.StatusOK)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	// Use a short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.ConfigDeploy(ctx, "test-fabric", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestConfigDeploy_MaxRetriesExhausted tests max retries exhausted
func TestConfigDeploy_MaxRetriesExhausted(t *testing.T) {
	var attempts int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		// Always fail with "deploy in progress"
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "Deploy is already in progress"}`))
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	// Use a context with enough time for retries
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := client.ConfigDeploy(ctx, "test-fabric", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "still in progress after") {
		t.Errorf("expected 'still in progress after' in error, got: %v", err)
	}

	// Should have attempted max retries (6)
	if atomic.LoadInt32(&attempts) != 6 {
		t.Errorf("expected 6 attempts, got %d", attempts)
	}
}

// TestConfigDeploy_ImmediateErrorNoRetry tests that non-retryable errors don't retry
func TestConfigDeploy_ImmediateErrorNoRetry(t *testing.T) {
	var attempts int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		// Return a different error that shouldn't trigger retry
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid fabric name"}`))
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.ConfigDeploy(context.Background(), "test-fabric", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should only attempt once - no retry for 400 errors
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

// TestConfigDeploy_ContextCanceled tests context cancellation during retry
func TestConfigDeploy_ContextCanceled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "Deploy is already in progress"}`))
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	// Cancel context quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.ConfigDeploy(ctx, "test-fabric", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

// TestIsDeployInProgress tests the deploy-in-progress detection
func TestIsDeployInProgress(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "plain error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name: "exact match",
			err: &APIError{
				StatusCode: 500,
				Body:       []byte(`Deploy is already in progress`),
			},
			expected: true,
		},
		{
			name: "case insensitive",
			err: &APIError{
				StatusCode: 500,
				Body:       []byte(`DEPLOY IS ALREADY IN PROGRESS`),
			},
			expected: true,
		},
		{
			name: "partial match - already in progress + deploy",
			err: &APIError{
				StatusCode: 500,
				Body:       []byte(`The deploy operation is already in progress for this fabric`),
			},
			expected: true,
		},
		{
			name: "partial match - config-deploy + in progress",
			err: &APIError{
				StatusCode: 500,
				Body:       []byte(`config-deploy is currently in progress`),
			},
			expected: true,
		},
		{
			name: "different error message",
			err: &APIError{
				StatusCode: 500,
				Body:       []byte(`Internal server error`),
			},
			expected: false,
		},
		{
			name: "400 error with deploy message",
			err: &APIError{
				StatusCode: 400,
				Body:       []byte(`Deploy is already in progress`),
			},
			expected: true, // We match on body regardless of status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeployInProgress(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDeleteSecurityGroup_Success tests successful security group deletion
// Note: Detach (PUT with attach=false) is best-effort and may fail validation
// if GroupName is not provided. The delete should still succeed.
func TestDeleteSecurityGroup_Success(t *testing.T) {
	var deleteCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delete is a DELETE to /groups?groupId=12345
		if r.Method == "DELETE" && strings.Contains(r.URL.Path, "/groups") {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		// Any other request (like failed detach attempt) - return error
		w.WriteHeader(http.StatusBadRequest)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.DeleteSecurityGroup(context.Background(), "test-fabric", 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}

// TestCreateSecurityContracts_Success tests successful contract creation
func TestCreateSecurityContracts_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BatchResponseContracts{
			BatchResponse: BatchResponse{
				TotalCount:   1,
				SuccessCount: 1,
			},
			SuccessList: []SecurityContract{
				{ContractName: "test-contract"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	contracts, err := client.CreateSecurityContracts(context.Background(), "test-fabric", []SecurityContract{
		{
			ContractName: "test-contract",
			Rules: []ContractRule{
				{Direction: "bidirectional", Action: "permit", ProtocolName: "default"},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(contracts))
	}
}

// TestCreateContractAssociations_Success tests successful association creation
func TestCreateContractAssociations_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BatchResponseAssociations{
			BatchResponse: BatchResponse{
				TotalCount:   1,
				SuccessCount: 1,
			},
			SuccessList: []ContractAssociation{
				{VRFName: "test-vrf", ContractName: "test-contract"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	srcID, dstID := 100, 200
	associations, err := client.CreateContractAssociations(context.Background(), "test-fabric", []ContractAssociation{
		{
			VRFName:      "test-vrf",
			SrcGroupID:   &srcID,
			DstGroupID:   &dstID,
			ContractName: "test-contract",
			Attach:       true,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(associations) != 1 {
		t.Fatalf("expected 1 association, got %d", len(associations))
	}
}

// TestValidation_EmptyFabricName tests validation for empty fabric name
func TestValidation_EmptyFabricName(t *testing.T) {
	client := &Client{} // No server needed for validation tests

	_, err := client.GetSecurityGroups(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty fabric name")
	}
	if !strings.Contains(err.Error(), "fabricName") {
		t.Errorf("expected error about fabricName, got: %v", err)
	}
}

// TestValidation_EmptyGroups tests validation for empty groups slice
func TestValidation_EmptyGroups(t *testing.T) {
	client := &Client{}

	_, err := client.CreateSecurityGroups(context.Background(), "test-fabric", []SecurityGroup{})
	if err == nil {
		t.Fatal("expected error for empty groups")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected 'cannot be empty' error, got: %v", err)
	}
}

// TestValidation_InvalidGroupID tests validation for invalid group ID
func TestValidation_InvalidGroupID(t *testing.T) {
	client := &Client{}

	err := client.DeleteSecurityGroup(context.Background(), "test-fabric", 0)
	if err == nil {
		t.Fatal("expected error for zero group ID")
	}
	if !strings.Contains(err.Error(), "groupID") {
		t.Errorf("expected error about groupID, got: %v", err)
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}

// TestUpdateSecurityGroups_Success tests successful security group update
func TestUpdateSecurityGroups_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		resp := BatchResponseGroups{
			BatchResponse: BatchResponse{
				TotalCount:   1,
				SuccessCount: 1,
			},
			SuccessList: []SecurityGroup{
				{GroupName: "updated-group", GroupID: intPtr(12345)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	groups, err := client.UpdateSecurityGroups(context.Background(), "test-fabric", []SecurityGroup{
		{GroupName: "updated-group", GroupID: intPtr(12345), Attach: false},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

// TestGetSecurityProtocols_Success tests successful protocol retrieval
func TestGetSecurityProtocols_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		protocols := []SecurityProtocol{
			{ProtocolName: "tcp", MatchType: "MatchAll"},
			{ProtocolName: "udp", MatchType: "MatchAll"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocols)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	protocols, err := client.GetSecurityProtocols(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(protocols) != 2 {
		t.Fatalf("expected 2 protocols, got %d", len(protocols))
	}
}

// TestGetSecurityContracts_Success tests successful contract retrieval
func TestGetSecurityContracts_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contracts := []SecurityContract{
			{ContractName: "contract1"},
			{ContractName: "contract2"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contracts)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	contracts, err := client.GetSecurityContracts(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contracts) != 2 {
		t.Fatalf("expected 2 contracts, got %d", len(contracts))
	}
}

// TestGetSecurityAssociations_Success tests successful association retrieval
func TestGetSecurityAssociations_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		associations := []ContractAssociation{
			{VRFName: "vrf1", ContractName: "contract1"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(associations)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	associations, err := client.GetSecurityAssociations(context.Background(), "test-fabric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(associations) != 1 {
		t.Fatalf("expected 1 association, got %d", len(associations))
	}
}

// TestDeleteSecurityContract_Success tests successful contract deletion
func TestDeleteSecurityContract_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.DeleteSecurityContract(context.Background(), "test-fabric", "test-contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDeleteSecurityAssociation_Success tests successful association deletion
func TestDeleteSecurityAssociation_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.DeleteSecurityAssociation(context.Background(), "test-fabric", "test-vrf", 100, 200, "test-contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDeleteSecurityProtocol_Success tests successful protocol deletion
func TestDeleteSecurityProtocol_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	err := client.DeleteSecurityProtocol(context.Background(), "test-fabric", "test-protocol")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestIsBadRequest tests IsBadRequest method
func TestIsBadRequest(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		expected bool
	}{
		{"400 error", &APIError{StatusCode: 400}, true},
		{"401 error", &APIError{StatusCode: 401}, false},
		{"500 error", &APIError{StatusCode: 500}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsBadRequest() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.err.IsBadRequest())
			}
		})
	}
}

// TestBatchError_Error tests BatchError.Error method
func TestBatchError_Error(t *testing.T) {
	err := &BatchError{
		Op:     "create groups",
		Fabric: "test-fabric",
		Failed: 2,
		Total:  3,
		Failures: []BatchItem{
			{Name: "group1", Code: "ERR1", Message: "invalid"},
			{Name: "group2", Code: "ERR2", Message: "duplicate"},
		},
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "create groups") {
		t.Errorf("expected operation in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "test-fabric") {
		t.Errorf("expected fabric in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "2/3 failed") {
		t.Errorf("expected failure count in error, got: %s", errStr)
	}
}

// TestBatchError_Error_NoFailures tests BatchError.Error with no failure details
func TestBatchError_Error_NoFailures(t *testing.T) {
	err := &BatchError{
		Op:      "create groups",
		Fabric:  "test-fabric",
		Failed:  1,
		Total:   2,
		Code:    "BATCH_ERR",
		Message: "batch failed",
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "batch failed") {
		t.Errorf("expected message in error, got: %s", errStr)
	}
}

// TestBatchError_FailureSummary tests BatchError.FailureSummary method
func TestBatchError_FailureSummary(t *testing.T) {
	err := &BatchError{
		Failures: []BatchItem{
			{Name: "group1", Code: "ERR1", Message: "invalid name"},
			{Name: "group2", Code: "ERR2", Message: "duplicate"},
		},
	}

	summary := err.FailureSummary(10)
	if !strings.Contains(summary, "group1") {
		t.Errorf("expected group1 in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "invalid name") {
		t.Errorf("expected message in summary, got: %s", summary)
	}
}

// TestCreateSecurityProtocols_Success tests successful protocol creation
func TestCreateSecurityProtocols_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Protocol creation returns []SecurityProtocol directly
		protocols := []SecurityProtocol{
			{ProtocolName: "custom-tcp", MatchType: "MatchAll"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocols)
	})

	client, server := newTestClient(t, handler)
	defer server.Close()

	protocols, err := client.CreateSecurityProtocols(context.Background(), "test-fabric", []SecurityProtocol{
		{ProtocolName: "custom-tcp", MatchType: "MatchAll"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(protocols) != 1 {
		t.Fatalf("expected 1 protocol, got %d", len(protocols))
	}
}
