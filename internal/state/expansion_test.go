package state

import (
	"os"
	"testing"
)

func TestExpansionState_RoundTrip(t *testing.T) {
	ws := NewWorkspaceDir(t.TempDir())
	want := ExpansionState{Callers: true, TypeDefs: false, Tests: true}
	if err := SaveExpansion(ws, want); err != nil {
		t.Fatalf("SaveExpansion: %v", err)
	}
	got, err := LoadExpansion(ws)
	if err != nil {
		t.Fatalf("LoadExpansion: %v", err)
	}
	if got != want {
		t.Fatalf("roundtrip: want %+v, got %+v", want, got)
	}
}

func TestExpansionState_MissingFileReturnsZero(t *testing.T) {
	ws := NewWorkspaceDir(t.TempDir())
	got, err := LoadExpansion(ws)
	if err != nil {
		t.Fatalf("missing file should be no error, got %v", err)
	}
	if (got != ExpansionState{}) {
		t.Fatalf("missing: want zero, got %+v", got)
	}
}

func TestExpansionState_CorruptFileReturnsZero(t *testing.T) {
	ws := NewWorkspaceDir(t.TempDir())
	if err := os.WriteFile(ws.ExpansionPath(), []byte("not json"), 0o600); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}
	got, err := LoadExpansion(ws)
	if err != nil {
		t.Fatalf("corrupt should fall back to zero, got %v", err)
	}
	if (got != ExpansionState{}) {
		t.Fatalf("corrupt: want zero, got %+v", got)
	}
}

func TestExpansionState_DeleteIdempotent(t *testing.T) {
	ws := NewWorkspaceDir(t.TempDir())
	if err := DeleteExpansion(ws); err != nil {
		t.Fatalf("delete on missing file: %v", err)
	}
	_ = SaveExpansion(ws, ExpansionState{Callers: true})
	if err := DeleteExpansion(ws); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := DeleteExpansion(ws); err != nil {
		t.Fatalf("second delete should be idempotent: %v", err)
	}
	if _, err := os.Stat(ws.ExpansionPath()); !os.IsNotExist(err) {
		t.Fatalf("expansion.json should be gone, stat err=%v", err)
	}
}
