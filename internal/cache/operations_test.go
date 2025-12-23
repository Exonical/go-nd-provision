package cache

import (
	"errors"
	"testing"
)

func TestErrKeyNotFound(t *testing.T) {
	err := ErrKeyNotFound
	if err.Error() != "key not found" {
		t.Errorf("expected 'key not found', got %s", err.Error())
	}
}

func TestErrLockNotAcquired(t *testing.T) {
	err := ErrLockNotAcquired
	if err.Error() != "lock not acquired" {
		t.Errorf("expected 'lock not acquired', got %s", err.Error())
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrKeyNotFound, ErrLockNotAcquired) {
		t.Error("ErrKeyNotFound and ErrLockNotAcquired should be distinct")
	}
}
