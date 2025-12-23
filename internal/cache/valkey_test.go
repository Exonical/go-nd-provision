package cache

import (
	"errors"
	"testing"
)

func TestErrCacheMiss(t *testing.T) {
	err := ErrCacheMiss
	if err.Error() != "cache miss" {
		t.Errorf("expected 'cache miss', got %s", err.Error())
	}
}

func TestErrCacheMiss_IsDistinct(t *testing.T) {
	if errors.Is(ErrCacheMiss, ErrKeyNotFound) {
		t.Error("ErrCacheMiss and ErrKeyNotFound should be distinct")
	}
}
