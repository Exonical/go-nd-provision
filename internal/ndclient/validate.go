package ndclient

import (
	"fmt"
	"strings"
)

// requireNonEmpty validates that a required string parameter is not empty
func requireNonEmpty(name, val string) error {
	if strings.TrimSpace(val) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
