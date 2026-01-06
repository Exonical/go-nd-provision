package common

import (
	"fmt"
	"strings"
)

// RequireNonEmpty returns an error if the value is empty or whitespace-only
func RequireNonEmpty(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
