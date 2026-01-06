package ndclient

import (
	"fmt"
	"strings"
)

// Operation name constants for consistent error messages
const (
	// Security Groups
	opCreateSecGroups = "create security groups"
	opGetSecGroups    = "get security groups"
	opGetSecGroup     = "get security group"
	opUpdateSecGroups = "update security groups"
	opDeleteSecGroup  = "delete security group"

	// Security Protocols
	opCreateSecProtocols = "create security protocols"
	opGetSecProtocols    = "get security protocols"
	opGetSecProtocol     = "get security protocol"
	opDeleteSecProtocol  = "delete security protocol"

	// Security Contracts
	opCreateSecContracts = "create security contracts"
	opGetSecContracts    = "get security contracts"
	opGetSecContract     = "get security contract"
	opUpdateSecContract  = "update security contract"
	opDeleteSecContract  = "delete security contract"

	// Contract Associations
	opCreateSecAssociations = "create contract associations"
	opGetSecAssociations    = "get contract associations"
	opDeleteSecAssociation  = "delete contract association"

	// Fabric Operations
	opConfigDeploy = "config deploy"
)

// BatchError represents a batch operation failure with full details
type BatchError struct {
	Op       string
	Fabric   string
	Failed   int
	Total    int
	Code     string // batch-level error code
	Message  string // batch-level error message
	Failures []BatchItem
}

func (e *BatchError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (fabric=%s): %d/%d failed", e.Op, e.Fabric, e.Failed, e.Total)

	// Include batch-level code/message if present
	if e.Code != "" {
		fmt.Fprintf(&b, "; code=%s", e.Code)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, "; msg=%s", compactMsg(e.Message, 100))
	}

	// Include first failure details if available
	if len(e.Failures) > 0 {
		f := e.Failures[0]
		b.WriteString("; first=[")
		if f.Name != "" {
			fmt.Fprintf(&b, "name=%s ", f.Name)
		}
		if f.ResourceID != "" {
			fmt.Fprintf(&b, "id=%s ", f.ResourceID)
		}
		if f.Code != "" {
			fmt.Fprintf(&b, "code=%s ", f.Code)
		}
		if f.Message != "" {
			fmt.Fprintf(&b, "msg=%s", compactMsg(f.Message, 80))
		}
		b.WriteString("]")
	}

	return b.String()
}

// IsPartial returns true if some items succeeded and some failed
func (e *BatchError) IsPartial() bool {
	return e.Failed > 0 && e.Failed < e.Total
}

// IsAllFailed returns true if all items in the batch failed
func (e *BatchError) IsAllFailed() bool {
	return e.Failed == e.Total && e.Total > 0
}

// FailureSummary returns a summary of failures (up to limit)
func (e *BatchError) FailureSummary(limit int) string {
	if len(e.Failures) == 0 {
		if e.Message != "" {
			return compactMsg(e.Message, 200)
		}
		return ""
	}

	if limit <= 0 || limit > len(e.Failures) {
		limit = len(e.Failures)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		f := e.Failures[i]
		name := f.Name
		if name == "" {
			name = f.ResourceID
		}
		parts = append(parts, fmt.Sprintf("%s:%s(%s)", name, f.Code, compactMsg(f.Message, 60)))
	}
	return strings.Join(parts, "; ")
}

// compactMsg strips newlines and caps length for log-friendly output
func compactMsg(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// isBatchError checks if the response indicates a batch-level or item-level error
func isBatchError(out BatchResponse) bool {
	if out.FailedCount > 0 {
		return true
	}
	// Check for batch-level error even when FailedCount==0
	if out.Code != "" {
		code := strings.ToLower(out.Code)
		if code != "success" && code != "ok" && code != "" {
			return true
		}
	}
	return false
}

// batchErr returns a BatchError if the batch operation had failures
func batchErr(op, fabric string, out BatchResponse) error {
	if !isBatchError(out) {
		return nil
	}
	return &BatchError{
		Op:       op,
		Fabric:   fabric,
		Failed:   out.FailedCount,
		Total:    out.TotalCount,
		Code:     out.Code,
		Message:  out.Message,
		Failures: out.FailureList,
	}
}
