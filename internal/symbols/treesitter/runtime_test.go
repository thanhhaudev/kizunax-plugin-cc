//go:build !lite

package treesitter

import (
	"context"
	"testing"
)

func TestRuntime_Init_SmokeOnly(t *testing.T) {
	ctx := context.Background()
	rt, err := getRuntime(ctx)
	if err != nil {
		t.Fatalf("getRuntime: %v", err)
	}
	if rt == nil {
		t.Fatal("getRuntime returned nil runtime")
	}
	// Verify singleton — second call returns same instance.
	rt2, err := getRuntime(ctx)
	if err != nil {
		t.Fatalf("getRuntime second call: %v", err)
	}
	if rt2 != rt {
		t.Fatal("getRuntime did not return singleton")
	}
}
