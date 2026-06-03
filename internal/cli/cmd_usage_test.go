package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

func TestDedupeUsageKeys_IncludesHelperKey_WhenDistinct(t *testing.T) {
	got := dedupeUsageKeys([]string{"kx_provider"}, []string{"kx_helper"})
	if len(got) != 2 {
		t.Fatalf("expected 2 keys, got %d (%v)", len(got), got)
	}
	if got[0] != "kx_provider" {
		t.Fatalf("provider key should be first, got %v", got)
	}
	if got[1] != "kx_helper" {
		t.Fatalf("helper key should be second, got %v", got)
	}
}

func TestDedupeUsageKeys_DedupesWhenHelperReusesProviderKey(t *testing.T) {
	got := dedupeUsageKeys([]string{"kx_shared"}, []string{"kx_shared"})
	if len(got) != 1 || got[0] != "kx_shared" {
		t.Fatalf("expected single deduped key, got %v", got)
	}
}

func TestDedupeUsageKeys_SkipsEmptyStrings(t *testing.T) {
	got := dedupeUsageKeys([]string{"kx_a", ""}, []string{"", "kx_b"})
	if len(got) != 2 {
		t.Fatalf("expected 2 keys (empties dropped), got %v", got)
	}
}

func TestDedupeUsageKeys_NilHelperPool(t *testing.T) {
	got := dedupeUsageKeys([]string{"kx_a", "kx_b"}, nil)
	if len(got) != 2 || got[0] != "kx_a" || got[1] != "kx_b" {
		t.Fatalf("nil helper pool: got %v", got)
	}
}

func TestDedupeUsageKeys_NilProviderPool(t *testing.T) {
	got := dedupeUsageKeys(nil, []string{"kx_helper"})
	if len(got) != 1 || got[0] != "kx_helper" {
		t.Fatalf("nil provider pool: got %v", got)
	}
}

func TestRenderUsageOutput_TwoKeys(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	usages := []usage.KeyUsage{
		{
			KeyMask: "kx_AAA…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "pro", Used: 1000, Limit: 50000, Remaining: 49000, ResetAt: now.Add(20 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Plan: "pro", Used: 100, Limit: 100000, Remaining: 99900, ResetAt: now.Add(27 * 24 * time.Hour)},
		},
		{KeyMask: "kx_BBB…", AuthFailed: true},
	}
	var buf bytes.Buffer
	writeUsageOutput(&buf, "openai", "round-robin", usages, now)
	got := buf.String()
	if !strings.Contains(got, "[1] kx_AAA…") || !strings.Contains(got, "[2] kx_BBB…") {
		t.Errorf("rows missing in output:\n%s", got)
	}
	if !strings.Contains(got, "auth failed") {
		t.Errorf("auth fail row missing:\n%s", got)
	}
	if !strings.Contains(got, "Provider: openai") {
		t.Errorf("provider header missing:\n%s", got)
	}
}
