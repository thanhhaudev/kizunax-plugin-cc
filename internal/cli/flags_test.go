package cli

import "testing"

func TestHasFlag_Quiet(t *testing.T) {
	if !hasFlag([]string{"--commit", "HEAD", "--quiet"}, "--quiet") {
		t.Error("expected --quiet detected")
	}
	if hasFlag([]string{"--commit", "HEAD"}, "--quiet") {
		t.Error("expected --quiet absent")
	}
}

func TestHasFlag_All(t *testing.T) {
	if !hasFlag([]string{"status", "--all"}, "--all") {
		t.Error("expected --all detected")
	}
	if hasFlag([]string{"status"}, "--all") {
		t.Error("expected --all absent")
	}
}
