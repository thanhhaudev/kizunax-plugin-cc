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
	ws := state.WorkspaceDir{Root: tmp}
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
