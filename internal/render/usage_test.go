package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

func TestAbbrevNum(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1.0k"},
		{4900, "4.9k"},
		{9999, "10.0k"},
		{10000, "10k"},
		{39000, "39k"},
		{999999, "1000k"},
		{1000000, "1.0M"},
		{1200000, "1.2M"},
	}
	for _, tc := range cases {
		got := abbrevNum(tc.n)
		if got != tc.want {
			t.Errorf("abbrevNum(%d): got %q want %q", tc.n, got, tc.want)
		}
	}
}

func TestAbbrevDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-1 * time.Second, "now"},
		{0, "now"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h"},
		{4 * time.Hour, "4h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{27 * 24 * time.Hour, "27d"},
	}
	for _, tc := range cases {
		got := abbrevDur(tc.d)
		if got != tc.want {
			t.Errorf("abbrevDur(%v): got %q want %q", tc.d, got, tc.want)
		}
	}
}

func loadGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(data)
}

func writeGolden(t *testing.T, name, got string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join("testdata", name), []byte(got), 0o600); err != nil {
		t.Fatalf("write golden %s: %v", name, err)
	}
}

// FrozenNow is a stable "now" anchor for golden tests so abbrevDur output is reproducible.
var FrozenNow = time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

func TestRenderUsage_TwoKeys(t *testing.T) {
	snap := usage.Snapshot{Provider: "openai", Usages: []usage.KeyUsage{
		{
			KeyMask: "kx_AbCd…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "pro", Used: 39000, Limit: 50000, Remaining: 11000, ResetAt: FrozenNow.Add(20 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Plan: "pro", Used: 35000, Limit: 100000, Remaining: 65000, ResetAt: FrozenNow.Add(27 * 24 * time.Hour)},
		},
		{
			KeyMask: "kx_EfGh…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "free", Used: 4900, Limit: 5000, Remaining: 100, ResetAt: FrozenNow.Add(3 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Plan: "free", Used: 8000, Limit: 100000, Remaining: 92000, ResetAt: FrozenNow.Add(27 * 24 * time.Hour)},
		},
	}}
	got := RenderUsageAt(snap, "round-robin", FrozenNow)
	if *updateGolden {
		writeGolden(t, "usage_two_keys.golden.md", got)
		return
	}
	want := loadGolden(t, "usage_two_keys.golden.md")
	if got != want {
		t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderUsage_AuthFail(t *testing.T) {
	snap := usage.Snapshot{Provider: "openai", Usages: []usage.KeyUsage{
		{
			KeyMask: "kx_AbCd…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "pro", Used: 39000, Limit: 50000, Remaining: 11000, ResetAt: FrozenNow.Add(20 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Plan: "pro", Used: 35000, Limit: 100000, Remaining: 65000, ResetAt: FrozenNow.Add(27 * 24 * time.Hour)},
		},
		{KeyMask: "kx_EfGh…", AuthFailed: true},
	}}
	got := RenderUsageAt(snap, "round-robin", FrozenNow)
	if *updateGolden {
		writeGolden(t, "usage_auth_fail.golden.md", got)
		return
	}
	want := loadGolden(t, "usage_auth_fail.golden.md")
	if got != want {
		t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderUsage_Unlimited(t *testing.T) {
	snap := usage.Snapshot{Provider: "openai", Usages: []usage.KeyUsage{
		{
			KeyMask: "kx_AbCd…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "enterprise", Used: 0, Limit: 999999, Remaining: 999999, Unlimited: true},
			Credits: &usage.Quota{Kind: "credits", Plan: "enterprise", Unlimited: true},
		},
	}}
	got := RenderUsageAt(snap, "round-robin", FrozenNow)
	if *updateGolden {
		writeGolden(t, "usage_unlimited.golden.md", got)
		return
	}
	want := loadGolden(t, "usage_unlimited.golden.md")
	if got != want {
		t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderUsage_PartialFail(t *testing.T) {
	snap := usage.Snapshot{Provider: "openai", Usages: []usage.KeyUsage{
		{
			KeyMask: "kx_EfGh…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "pro", Used: 39000, Limit: 50000, Remaining: 11000, ResetAt: FrozenNow.Add(20 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Err: "http 503: down for maintenance"},
		},
	}}
	got := RenderUsageAt(snap, "round-robin", FrozenNow)
	if *updateGolden {
		writeGolden(t, "usage_partial_fail.golden.md", got)
		return
	}
	want := loadGolden(t, "usage_partial_fail.golden.md")
	if got != want {
		t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
