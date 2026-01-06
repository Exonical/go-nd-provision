package common

import (
	"errors"
	"fmt"
)

// APIErrorWithBody is an interface for errors that contain an API response body.
// This allows consistent error formatting across different client packages.
type APIErrorWithBody interface {
	error
	BodyString(limit int) string
}

// WrapAPIError wraps an error with operation context.
// If the error contains an API response body (implements APIErrorWithBody),
// it includes the truncated body in the error message for debugging.
func WrapAPIError(op string, err error) error {
	if err == nil {
		return nil
	}

	// Check if error (or wrapped error) has a body
	var apiErr APIErrorWithBody
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s: %w, body: %s", op, err, apiErr.BodyString(500))
	}
	return fmt.Errorf("%s: %w", op, err)
}

// WrapAPIErrorWithContext wraps an error with operation and context (e.g., fabric name).
// If the error contains an API response body, it includes the truncated body.
func WrapAPIErrorWithContext(op, context string, err error) error {
	if err == nil {
		return nil
	}

	// Check if error (or wrapped error) has a body
	var apiErr APIErrorWithBody
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s (%s): %w, body: %s", op, context, err, apiErr.BodyString(500))
	}
	return fmt.Errorf("%s (%s): %w", op, context, err)
}
