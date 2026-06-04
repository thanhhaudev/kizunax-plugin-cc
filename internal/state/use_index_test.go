package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUseIndexState_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	want := UseIndexState{Enabled: true}
	if err := SaveUseIndex(ws, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadUseIndex(ws)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled: got %v want %v", got.Enabled, want.Enabled)
	}
}

func TestUseIndexState_MissingFileTreatedAsDisabled(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	got, err := LoadUseIndex(ws)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if got.Enabled {
		t.Errorf("missing file should be disabled, got Enabled=true")
	}
}

func TestUseIndexState_CorruptFileTreatedAsDisabled(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	if err := os.WriteFile(ws.UseIndexPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadUseIndex(ws)
	if err != nil {
		t.Fatalf("load corrupt: %v", err)
	}
	if got.Enabled {
		t.Errorf("corrupt file should be disabled, got Enabled=true")
	}
}

func TestUseIndexState_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	ws := NewWorkspaceDir(tmp)

	want := UseIndexState{Enabled: true}
	if err := SaveUseIndex(ws, want); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(filepath.Dir(ws.UseIndexPath()))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}
