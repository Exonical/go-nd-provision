package common

import "fmt"

// RequireNonEmpty returns an error if the value is empty
func RequireNonEmpty(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}
