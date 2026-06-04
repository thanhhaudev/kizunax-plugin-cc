package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStopGateState_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	want := StopGateState{
		Enabled:            true,
		LastHash:           []byte{0x01, 0x02, 0x03, 0x04},
		LastRunAt:          time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
		LastVerdictHadHigh: true,
		LastResult: &CachedVerdict{
			Findings: []CachedFinding{
				{Severity: "high", File: "api.go", Line: 42, Title: "missing error wrap"},
			},
		},
	}
	if err := SaveStopGate(ws, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadStopGate(ws)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled: got %v want %v", got.Enabled, want.Enabled)
	}
	if !bytes.Equal(got.LastHash, want.LastHash) {
		t.Errorf("LastHash mismatch")
	}
	if !got.LastRunAt.Equal(want.LastRunAt) {
		t.Errorf("LastRunAt: got %v want %v", got.LastRunAt, want.LastRunAt)
	}
	if got.LastVerdictHadHigh != want.LastVerdictHadHigh {
		t.Errorf("LastVerdictHadHigh: got %v want %v", got.LastVerdictHadHigh, want.LastVerdictHadHigh)
	}
	if got.LastResult == nil || len(got.LastResult.Findings) != 1 {
		t.Fatalf("LastResult: got %+v", got.LastResult)
	}
	if got.LastResult.Findings[0].File != "api.go" {
		t.Errorf("Finding file mismatch")
	}
}

func TestStopGateState_MissingFileTreatedAsDisabled(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	got, err := LoadStopGate(ws)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if got.Enabled {
		t.Errorf("missing file should be disabled, got Enabled=true")
	}
}

func TestStopGateState_CorruptFileTreatedAsDisabled(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	if err := os.WriteFile(ws.StopGatePath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadStopGate(ws)
	if err != nil {
		t.Fatalf("load corrupt: %v", err)
	}
	if got.Enabled {
		t.Errorf("corrupt file should be disabled, got Enabled=true")
	}
}

func TestStopGateState_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	want := StopGateState{Enabled: true, LastRunAt: time.Now().UTC()}
	if err := SaveStopGate(ws, want); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(filepath.Dir(ws.StopGatePath()))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}
