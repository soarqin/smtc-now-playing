//go:build windows

package smtc

import (
	"testing"
)

// TestReadNullableBool_NilSafe verifies that readNullableBool safely handles nil IReference.
func TestReadNullableBool_NilSafe(t *testing.T) {
	val, ok := readNullableBool(nil)
	if val != false || ok != false {
		t.Errorf("readNullableBool(nil) = (%v, %v), want (false, false)", val, ok)
	}
}

// TestReadNullableFloat64_NilSafe verifies that readNullableFloat64 safely handles nil IReference.
func TestReadNullableFloat64_NilSafe(t *testing.T) {
	val, ok := readNullableFloat64(nil)
	if val != 0.0 || ok != false {
		t.Errorf("readNullableFloat64(nil) = (%v, %v), want (0.0, false)", val, ok)
	}
}

// TestReadNullableInt32_NilSafe verifies that readNullableInt32 safely handles nil IReference.
func TestReadNullableInt32_NilSafe(t *testing.T) {
	val, ok := readNullableInt32(nil)
	if val != 0 || ok != false {
		t.Errorf("readNullableInt32(nil) = (%v, %v), want (0, false)", val, ok)
	}
}
