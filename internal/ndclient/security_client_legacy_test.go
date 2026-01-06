package ndclient

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWrapOpErr_NilError(t *testing.T) {
	result := wrapOpErr("test op", "test-fabric", nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestWrapOpErr_PlainError(t *testing.T) {
	plainErr := errors.New("connection refused")
	result := wrapOpErr("get security groups", "my-fabric", plainErr)

	if result == nil {
		t.Fatal("expected non-nil error")
	}

	// Should contain operation and fabric
	errStr := result.Error()
	if !strings.Contains(errStr, "get security groups") {
		t.Errorf("expected error to contain operation, got: %s", errStr)
	}
	if !strings.Contains(errStr, "fabric=my-fabric") {
		t.Errorf("expected error to contain fabric, got: %s", errStr)
	}
	if !strings.Contains(errStr, "connection refused") {
		t.Errorf("expected error to contain original message, got: %s", errStr)
	}
	// Should NOT contain "body:" since it's not an APIError
	if strings.Contains(errStr, "body:") {
		t.Errorf("plain error should not have body, got: %s", errStr)
	}

	// Should unwrap to original error
	if !errors.Is(result, plainErr) {
		t.Error("expected result to unwrap to original error")
	}
}

func TestWrapOpErr_APIError(t *testing.T) {
	apiErr := &APIError{
		Method:     "POST",
		Path:       "/api/security/groups",
		StatusCode: 400,
		Body:       []byte(`{"error": "invalid group name"}`),
	}

	result := wrapOpErr("create security groups", "prod-fabric", apiErr)

	if result == nil {
		t.Fatal("expected non-nil error")
	}

	errStr := result.Error()
	// Should contain operation and fabric
	if !strings.Contains(errStr, "create security groups") {
		t.Errorf("expected error to contain operation, got: %s", errStr)
	}
	if !strings.Contains(errStr, "fabric=prod-fabric") {
		t.Errorf("expected error to contain fabric, got: %s", errStr)
	}
	// Should contain body snippet
	if !strings.Contains(errStr, "body:") {
		t.Errorf("expected error to contain body, got: %s", errStr)
	}
	if !strings.Contains(errStr, "invalid group name") {
		t.Errorf("expected error to contain body content, got: %s", errStr)
	}

	// Should unwrap to APIError
	var unwrapped *APIError
	if !errors.As(result, &unwrapped) {
		t.Error("expected result to unwrap to APIError")
	}
	if unwrapped.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", unwrapped.StatusCode)
	}
}

func TestWrapOpErr_WrappedAPIError(t *testing.T) {
	// This tests the errors.As behavior - APIError wrapped in another error
	apiErr := &APIError{
		Method:     "DELETE",
		Path:       "/api/security/groups",
		StatusCode: 404,
		Body:       []byte(`{"error": "group not found"}`),
	}
	wrappedErr := fmt.Errorf("outer context: %w", apiErr)

	result := wrapOpErr("delete security group", "test-fabric", wrappedErr)

	if result == nil {
		t.Fatal("expected non-nil error")
	}

	errStr := result.Error()
	// Should still extract body from wrapped APIError via errors.As
	if !strings.Contains(errStr, "body:") {
		t.Errorf("expected error to contain body even when wrapped, got: %s", errStr)
	}
	if !strings.Contains(errStr, "group not found") {
		t.Errorf("expected error to contain body content, got: %s", errStr)
	}

	// Should still be able to unwrap to APIError
	var unwrapped *APIError
	if !errors.As(result, &unwrapped) {
		t.Error("expected result to unwrap to APIError")
	}
	if unwrapped.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", unwrapped.StatusCode)
	}
}

func TestWrapOpErr_LongBody_Truncated(t *testing.T) {
	longBody := strings.Repeat("x", 1000)
	apiErr := &APIError{
		Method:     "POST",
		Path:       "/api/security/contracts",
		StatusCode: 500,
		Body:       []byte(longBody),
	}

	result := wrapOpErr("create security contracts", "fabric", apiErr)
	errStr := result.Error()

	// Body should be truncated to 500 chars + ellipsis
	if len(errStr) > 700 { // op + fabric + body(500) + overhead
		t.Errorf("expected truncated body, got length %d", len(errStr))
	}
	if !strings.Contains(errStr, "…") {
		t.Errorf("expected truncation marker, got: %s", errStr)
	}
}

func TestAPIError_BodyString(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		limit    int
		expected string
	}{
		{"short body no limit", "hello", 0, "hello"},
		{"short body with limit", "hello", 10, "hello"},
		{"exact limit", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello…"},
		{"empty body", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &APIError{Body: []byte(tt.body)}
			result := apiErr.BodyString(tt.limit)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("not found"), false},
		{"APIError 404", &APIError{StatusCode: 404}, true},
		{"APIError 400", &APIError{StatusCode: 400}, false},
		{"wrapped APIError 404", fmt.Errorf("wrapped: %w", &APIError{StatusCode: 404}), true},
		{"wrapped APIError 500", fmt.Errorf("wrapped: %w", &APIError{StatusCode: 500}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsConflictError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("conflict"), false},
		{"APIError 409", &APIError{StatusCode: 409}, true},
		{"APIError 400", &APIError{StatusCode: 400}, false},
		{"wrapped APIError 409", fmt.Errorf("wrapped: %w", &APIError{StatusCode: 409}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConflictError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test that operation constants are defined and non-empty
func TestOperationConstants(t *testing.T) {
	ops := []string{
		opCreateSecGroups,
		opGetSecGroups,
		opGetSecGroup,
		opUpdateSecGroups,
		opDeleteSecGroup,
		opCreateSecProtocols,
		opGetSecProtocols,
		opGetSecProtocol,
		opDeleteSecProtocol,
		opCreateSecContracts,
		opGetSecContracts,
		opGetSecContract,
		opUpdateSecContract,
		opDeleteSecContract,
		opCreateSecAssociations,
		opGetSecAssociations,
		opDeleteSecAssociation,
		opConfigDeploy,
	}

	for _, op := range ops {
		if op == "" {
			t.Errorf("operation constant should not be empty")
		}
	}
}
