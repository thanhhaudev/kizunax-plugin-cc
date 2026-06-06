package runner

import (
	"strings"
	"testing"
)

func TestCombinedContext_BothEmpty(t *testing.T) {
	if got := combinedContext("", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCombinedContext_FileOnly(t *testing.T) {
	if got := combinedContext("FILE", ""); got != "FILE" {
		t.Errorf("expected FILE, got %q", got)
	}
}

func TestCombinedContext_InlineOnly(t *testing.T) {
	got := combinedContext("", "INLINE")
	if !strings.Contains(got, "Per-review notes") {
		t.Errorf("expected per-review header, got %q", got)
	}
	if !strings.Contains(got, "INLINE") {
		t.Errorf("expected inline body, got %q", got)
	}
}

func TestCombinedContext_BothPresent_FileFirst(t *testing.T) {
	got := combinedContext("FILE-BODY", "INLINE-BODY")
	fi := strings.Index(got, "FILE-BODY")
	ii := strings.Index(got, "INLINE-BODY")
	if fi < 0 || ii < 0 {
		t.Fatalf("missing markers in %q", got)
	}
	if fi >= ii {
		t.Errorf("file must appear before inline: fi=%d ii=%d", fi, ii)
	}
	if !strings.Contains(got, "---") {
		t.Errorf("expected separator between file and inline, got %q", got)
	}
}
