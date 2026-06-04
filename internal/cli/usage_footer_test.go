package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

func makeWS(t *testing.T) state.WorkspaceDir {
	t.Helper()
	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	return ws
}

func seedCache(t *testing.T, ws state.WorkspaceDir, key string, coding, credits *usage.Quota, age time.Duration) {
	t.Helper()
	h := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(h[:])
	entry := usage.KeyUsage{
		KeyHash:   hash,
		KeyMask:   "kx_X…",
		Coding:    coding,
		Credits:   credits,
		FetchedAt: time.Now().Add(-age),
	}
	cache := map[string]usage.KeyUsage{hash: entry}
	data, _ := json.MarshalIndent(map[string]any{"entries": cache}, "", "  ")
	if err := os.WriteFile(filepath.Join(ws.Root, "usage.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAppendUsageFooter_CacheMiss(t *testing.T) {
	ws := makeWS(t)
	var buf bytes.Buffer
	appendUsageFooter(&buf, ws, "kx_NONE")
	if buf.Len() != 0 {
		t.Errorf("cache miss should write nothing, got %q", buf.String())
	}
}

func TestAppendUsageFooter_FreshNotLow(t *testing.T) {
	ws := makeWS(t)
	seedCache(t, ws, "kx_OK",
		&usage.Quota{Kind: "coding", Plan: "pro", Used: 100, Limit: 1000, Remaining: 900},
		&usage.Quota{Kind: "credits", Plan: "pro", Used: 100, Limit: 1000, Remaining: 900},
		10*time.Second,
	)
	var buf bytes.Buffer
	appendUsageFooter(&buf, ws, "kx_OK")
	if buf.Len() != 0 {
		t.Errorf("not-low should write nothing, got %q", buf.String())
	}
}

func TestAppendUsageFooter_StaleSkipped(t *testing.T) {
	ws := makeWS(t)
	seedCache(t, ws, "kx_STALE",
		&usage.Quota{Kind: "coding", Plan: "free", Used: 999, Limit: 1000, Remaining: 1, ResetAt: time.Now().Add(2 * time.Minute)},
		nil,
		2*time.Minute, // > 60s TTL
	)
	var buf bytes.Buffer
	appendUsageFooter(&buf, ws, "kx_STALE")
	if buf.Len() != 0 {
		t.Errorf("stale cache should be ignored, got %q", buf.String())
	}
}

func TestAppendUsageFooter_FreshLow(t *testing.T) {
	ws := makeWS(t)
	// Remaining=4 triggers Coding absolute floor (<5).
	seedCache(t, ws, "kx_LOW",
		&usage.Quota{Kind: "coding", Plan: "free", Used: 4996, Limit: 5000, Remaining: 4, ResetAt: time.Now().Add(3 * time.Minute)},
		nil,
		5*time.Second,
	)
	var buf bytes.Buffer
	appendUsageFooter(&buf, ws, "kx_LOW")
	got := buf.String()
	if got == "" {
		t.Fatalf("fresh-low should write footer")
	}
	if !bytes.Contains([]byte(got), []byte("⚠️")) {
		t.Errorf("footer missing warn marker:\n%s", got)
	}
	// Verify the key mask is populated (live MaskKey result), not empty from cache strip.
	if !bytes.Contains([]byte(got), []byte("kx_LOW…")) {
		t.Errorf("footer should show live MaskKey(\"kx_LOW\") = \"kx_LOW…\":\n%s", got)
	}
}

// TestAppendUsageFooterIfNotQuiet_LowQuotaSnapshotProducesWarning is a
// regression guard for the v0.9-era report that the footer was missing on
// direct binary invocation of `kizunax review`. The bug could not be
// reproduced in v0.10 — v0.9 T16 (foreground job persist) added a
// synchronous RefreshAndWait before the footer call in cmd_review.go, which
// keeps the cache fresh enough for the footer logic to fire on direct
// invocation just as it does under the slash-command flow.
//
// This test exercises the same surface from a DIFFERENT angle than the
// existing seedCache-based tests: it goes through usage.SaveCache (the
// production write path that RefreshAndWait uses) instead of writing raw
// JSON. That way, if either side of the contract (writer or reader) ever
// drifts so the cache shape mismatches the footer's expectations, this
// test fails fast.
func TestAppendUsageFooterIfNotQuiet_LowQuotaSnapshotProducesWarning(t *testing.T) {
	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	const apiKey = "kx_test1234"
	now := time.Now()
	snap := usage.Snapshot{
		Provider: "anthropic",
		Usages: []usage.KeyUsage{
			{
				KeyHash:   usage.HashKey(apiKey),
				KeyMask:   usage.MaskKey(apiKey),
				FetchedAt: now,
				Coding: &usage.Quota{
					Kind:      "coding",
					Plan:      "free",
					Used:      48,
					Limit:     50,
					Remaining: 2, // < absolute-5 coding floor → IsLow true
					ResetAt:   now.Add(2 * time.Hour),
				},
			},
		},
	}
	if err := usage.SaveCache(ws, snap); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	var buf bytes.Buffer
	appendUsageFooterIfNotQuiet(&buf, false /*quiet*/, ws, apiKey)
	out := buf.String()
	if out == "" {
		t.Fatalf("expected low-quota warning footer, got empty output")
	}
	if !bytes.Contains([]byte(out), []byte("⚠️")) {
		t.Errorf("footer missing warn glyph:\n%s", out)
	}
	// Confirm MaskKey is repopulated live (cache strips it on round-trip).
	if !bytes.Contains([]byte(out), []byte(usage.MaskKey(apiKey))) {
		t.Errorf("footer should carry live mask %q:\n%s", usage.MaskKey(apiKey), out)
	}
}

func TestAppendUsageFooterByHash_LookupBypassesConfigLoad(t *testing.T) {
	ws := makeWS(t)
	workerKey := "kx_WORKER"
	seedCache(t, ws, workerKey,
		&usage.Quota{Kind: "coding", Plan: "free", Used: 4996, Limit: 5000, Remaining: 4, ResetAt: time.Now().Add(3 * time.Minute)},
		nil,
		5*time.Second,
	)

	// Simulate `kizunax result` reading by the persisted KeyHash — what
	// runResult does now. The actual API key is never seen here, only the hash
	// and mask that were stored in job.Request.
	hash := usage.HashKey(workerKey)
	mask := usage.MaskKey(workerKey)

	var buf bytes.Buffer
	appendUsageFooterByHash(&buf, ws, hash, mask)
	got := buf.String()
	if got == "" {
		t.Fatalf("by-hash lookup should write footer")
	}
	if !bytes.Contains([]byte(got), []byte("kx_WORK…")) {
		t.Errorf("footer should carry the persisted mask:\n%s", got)
	}

	// A different key's hash → cache miss → silent.
	var buf2 bytes.Buffer
	appendUsageFooterByHash(&buf2, ws, usage.HashKey("kx_OTHER"), "kx_OTHE…")
	if buf2.Len() != 0 {
		t.Errorf("mismatched hash should be a cache miss, got: %q", buf2.String())
	}

	// Empty hash → silent.
	var buf3 bytes.Buffer
	appendUsageFooterByHash(&buf3, ws, "", "")
	if buf3.Len() != 0 {
		t.Errorf("empty hash should be silent, got: %q", buf3.String())
	}
}
