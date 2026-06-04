// internal/usage/cache_test.go
package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
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

func TestSaveLoadCache_RoundTrip(t *testing.T) {
	ws := makeWS(t)
	snap := Snapshot{Usages: []KeyUsage{
		{
			KeyHash:   hashKey("kx_ABCD"),
			KeyMask:   "kx_AB…",
			Coding:    &Quota{Kind: "coding", Plan: "pro", Used: 100, Limit: 500, Remaining: 400, ResetAt: time.Unix(1700000000, 0).UTC()},
			Credits:   &Quota{Kind: "credits", Plan: "pro", Used: 1000, Limit: 100000, Remaining: 99000},
			FetchedAt: time.Unix(1690000000, 0).UTC(),
		},
	}}
	if err := SaveCache(ws, snap); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}
	got, err := LoadCache(ws)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	entry, ok := got[hashKey("kx_ABCD")]
	if !ok {
		t.Fatalf("entry not found")
	}
	if entry.Coding == nil || entry.Coding.Used != 100 {
		t.Errorf("coding not round-tripped")
	}
	if entry.Coding.Kind != "coding" {
		t.Errorf("Coding.Kind lost on round-trip: %q (needed for IsLow's coding absolute floor)", entry.Coding.Kind)
	}
	if entry.Credits == nil || entry.Credits.Remaining != 99000 {
		t.Errorf("credits not round-tripped")
	}
	if entry.Credits.Kind != "credits" {
		t.Errorf("Credits.Kind lost on round-trip: %q", entry.Credits.Kind)
	}
}

func TestSaveCache_SkipsFailedQuotas(t *testing.T) {
	ws := makeWS(t)
	snap := Snapshot{Usages: []KeyUsage{
		{
			KeyHash:   hashKey("kx_FAIL"),
			Coding:    &Quota{Err: "timeout"},
			Credits:   &Quota{Kind: "credits", Used: 5, Limit: 100, Remaining: 95},
			FetchedAt: time.Now().UTC(),
		},
	}}
	_ = SaveCache(ws, snap)
	got, _ := LoadCache(ws)
	entry := got[hashKey("kx_FAIL")]
	if entry.Coding != nil {
		t.Errorf("failed Coding should not persist")
	}
	if entry.Credits == nil {
		t.Errorf("successful Credits should persist")
	}
}

func TestSaveCache_SkipsFullAuthFailKeys(t *testing.T) {
	ws := makeWS(t)
	snap := Snapshot{Usages: []KeyUsage{
		{KeyHash: hashKey("kx_AUTHFAIL"), AuthFailed: true, FetchedAt: time.Now()},
	}}
	_ = SaveCache(ws, snap)
	got, _ := LoadCache(ws)
	if _, ok := got[hashKey("kx_AUTHFAIL")]; ok {
		t.Errorf("auth-failed key should not be in cache")
	}
}

func TestLoadCache_MissingFile(t *testing.T) {
	ws := makeWS(t)
	got, err := LoadCache(ws)
	if err != nil {
		t.Errorf("missing file should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestLoadCache_CorruptFile(t *testing.T) {
	ws := makeWS(t)
	path := filepath.Join(ws.Root, "usage.json")
	if err := os.WriteFile(path, []byte("not json {{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCache(ws)
	if err != nil {
		t.Errorf("corrupt file should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("corrupt file should yield empty map")
	}
}

func TestLoadCachedEntry_FreshAndStale(t *testing.T) {
	ws := makeWS(t)
	key := "kx_TIME"
	now := time.Now().UTC()
	snap := Snapshot{Usages: []KeyUsage{{
		KeyHash:   hashKey(key),
		Credits:   &Quota{Kind: "credits", Used: 1, Limit: 100, Remaining: 99},
		FetchedAt: now.Add(-59 * time.Second),
	}}}
	_ = SaveCache(ws, snap)

	entry, fresh := LoadCachedEntry(ws, key)
	if !fresh {
		t.Errorf("59s-old entry should be fresh")
	}
	if entry.Credits == nil {
		t.Errorf("entry should have credits")
	}

	// rewrite with stale timestamp
	snap.Usages[0].FetchedAt = now.Add(-61 * time.Second)
	_ = SaveCache(ws, snap)
	_, fresh = LoadCachedEntry(ws, key)
	if fresh {
		t.Errorf("61s-old entry should be stale")
	}
}
