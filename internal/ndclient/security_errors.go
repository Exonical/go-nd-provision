package ndclient

import (
	"fmt"
	"strings"
)

// Operation name constants for consistent error messages
const (
	opCreateSecGroups       = "create security groups"
	opCreateSecContracts    = "create security contracts"
	opCreateSecAssociations = "create contract associations"
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
	if len(e.Failures) == 0 {
		if e.Message != "" {
			return fmt.Sprintf("%s (fabric=%s): %d/%d failed; code=%q msg=%q", e.Op, e.Fabric, e.Failed, e.Total, e.Code, e.Message)
		}
		return fmt.Sprintf("%s (fabric=%s): %d/%d failed", e.Op, e.Fabric, e.Failed, e.Total)
	}
	f := e.Failures[0]
	return fmt.Sprintf("%s (fabric=%s): %d/%d failed; first name=%q id=%q code=%q msg=%q",
		e.Op, e.Fabric, e.Failed, e.Total, f.Name, f.ResourceID, f.Code, f.Message)
}

// FailureSummary returns a summary of all failures (up to limit)
func (e *BatchError) FailureSummary(limit int) string {
	if limit <= 0 || limit > len(e.Failures) {
		limit = len(e.Failures)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		f := e.Failures[i]
		parts = append(parts, fmt.Sprintf("%s:%s(%s)", f.Name, f.Code, f.Message))
	}
	return strings.Join(parts, "; ")
}

// batchErr returns a BatchError if the batch operation had failures
func batchErr(op, fabric string, out BatchResponse) error {
	if out.FailedCount > 0 {
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
	return nil
}
