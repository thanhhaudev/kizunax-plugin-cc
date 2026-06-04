package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSON_AtomicAndReadable(t *testing.T) {
	dir := t.TempDir()
	idx := &Index{
		Version: CurrentSchemaVersion,
		Root:    dir,
		Built:   1234567890,
		Files: map[string]*FileIndex{
			"a.go": {Path: "a.go", Lang: "go", Defs: []Location{
				{Name: "Foo", File: "a.go", Line: 10, Kind: SymDef},
			}},
		},
	}
	path := filepath.Join(dir, "index.json")
	if err := WriteJSON(path, idx); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("want mode 0600, got %v", info.Mode().Perm())
	}
}

func TestLoadJSON_RoundtripWithLookups(t *testing.T) {
	dir := t.TempDir()
	original := &Index{
		Version: CurrentSchemaVersion,
		Root:    dir,
		Files: map[string]*FileIndex{
			"x.go": {Path: "x.go", Lang: "go", Defs: []Location{
				{Name: "Bar", File: "x.go", Line: 5, Kind: SymDef},
			}},
		},
	}
	path := filepath.Join(dir, "index.json")
	if err := WriteJSON(path, original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	loaded, err := LoadJSON(path)
	if err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	if !loaded.Healthy() {
		t.Fatalf("loaded index not healthy")
	}
	got := loaded.LookupDefs("Bar", "")
	if len(got) != 1 || got[0].File != "x.go" {
		t.Fatalf("LookupDefs Bar: want 1 from x.go, got %+v", got)
	}
}

func TestLoadJSON_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	stale := &Index{
		Version: CurrentSchemaVersion + 99, // future version
		Files:   map[string]*FileIndex{},
	}
	path := filepath.Join(dir, "index.json")
	if err := WriteJSON(path, stale); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	_, err := LoadJSON(path)
	if err == nil {
		t.Fatalf("expected ErrSchemaVersionMismatch, got nil")
	}
	if !IsSchemaVersionMismatch(err) {
		t.Fatalf("expected schema mismatch err, got %v", err)
	}
}

func TestLoadJSON_NotExist(t *testing.T) {
	_, err := LoadJSON(filepath.Join(t.TempDir(), "no-such-file.json"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.IsNotExist, got %v", err)
	}
}
