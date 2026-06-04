package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFile_GoExtractsDefsAndRefs(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`package x

func Authenticate(user string) error {
	return nil
}

func caller() {
	Authenticate("foo")
}
`)
	path := filepath.Join(dir, "auth.go")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fi, err := ScanFile(dir, "auth.go")
	if err != nil {
		t.Fatalf("ScanFile: %v", err)
	}
	if fi.Path != "auth.go" {
		t.Fatalf("Path: want auth.go, got %s", fi.Path)
	}
	if fi.Lang != "go" {
		t.Fatalf("Lang: want go, got %s", fi.Lang)
	}
	if fi.Mtime == 0 {
		t.Fatalf("Mtime: want non-zero, got 0")
	}

	// Defs should include "Authenticate" (and "caller").
	defNames := names(fi.Defs)
	if !contains(defNames, "Authenticate") {
		t.Fatalf("Defs missing Authenticate: %v", defNames)
	}

	// Refs should include the call to Authenticate inside caller().
	refNames := names(fi.Refs)
	if !contains(refNames, "Authenticate") {
		t.Fatalf("Refs missing Authenticate call: %v", refNames)
	}
}

func TestScanFile_UnsupportedExtensionSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	os.WriteFile(path, []byte("hello"), 0o644)

	fi, err := ScanFile(dir, "notes.txt")
	if err != nil {
		t.Fatalf("ScanFile: %v", err)
	}
	if fi != nil {
		t.Fatalf("expected nil FileIndex for unsupported ext, got %+v", fi)
	}
}

func TestScanFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ScanFile(dir, "no-such-file.go")
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

// Test helpers
func names(locs []Location) []string {
	out := make([]string, len(locs))
	for i, l := range locs {
		out[i] = l.Name
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
