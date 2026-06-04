package bundlelog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func TestEnabled_RespectsEnvVar(t *testing.T) {
	t.Setenv("KIZUNAX_BUNDLE_LOG", "")
	if Enabled() {
		t.Fatalf("Enabled() must be false when env is empty")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "1")
	if !Enabled() {
		t.Fatalf("Enabled() must be true when env=1")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "true")
	if !Enabled() {
		t.Fatalf("Enabled() must be true when env=true")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "0")
	if Enabled() {
		t.Fatalf("Enabled() must be false when env=0")
	}
}

func TestAppend_WritesValidJSONL(t *testing.T) {
	ws := state.WorkspaceDir{Root: t.TempDir()}
	entry := Entry{
		Timestamp: "2026-06-04T10:00:00Z",
		Workspace: "test-ws",
		DiffFiles: 3,
		Bundle: []diff.ReferencedFileLogEntry{
			{Path: "a.go", Reason: "diff_file", Bytes: 100},
		},
		Stats: Stats{Extracted: 14, Filtered: 12, Resolved: 7, Attached: 4, Dropped: 2, BudgetBytes: 32768, UsedBytes: 6347},
	}
	Append(ws, entry)

	path := filepath.Join(ws.Root, LogName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d (%q)", len(lines), string(data))
	}
	var got Entry
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("invalid jsonl: %v\nline: %s", err, lines[0])
	}
	if got.Stats.Extracted != 14 || got.Stats.Filtered != 12 || got.Stats.Resolved != 7 {
		t.Fatalf("stats roundtrip mismatch: %+v", got.Stats)
	}
}

func TestAppend_AppendsMultipleEntries(t *testing.T) {
	ws := state.WorkspaceDir{Root: t.TempDir()}
	for i := 0; i < 3; i++ {
		Append(ws, Entry{Timestamp: "2026-06-04T10:00:00Z", Workspace: "x"})
	}
	path := filepath.Join(ws.Root, LogName)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("line %d invalid: %v", count, err)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("want 3 lines, got %d", count)
	}
}

func TestAppend_EmptyWorkspaceRootSkips(t *testing.T) {
	// Root == "" → no path to write to. Must not panic, must not create a file.
	ws := state.WorkspaceDir{Root: ""}
	Append(ws, Entry{Timestamp: "2026-06-04T10:00:00Z"})
	// Nothing to assert beyond "did not panic and did not write anywhere
	// meaningful". This test is here to lock in the guard.
}

func TestAppend_RotatesAtSizeCap(t *testing.T) {
	ws := state.WorkspaceDir{Root: t.TempDir()}
	path := filepath.Join(ws.Root, LogName)
	backup := filepath.Join(ws.Root, BackupName)

	// Pre-fill log file to >= SizeCapBytes with a single big line so the
	// next Append() triggers rotation.
	big := make([]byte, SizeCapBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	Append(ws, Entry{Timestamp: "2026-06-04T10:00:00Z", Workspace: "rotate-test"})

	// Backup must now exist with the old big content.
	bData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if len(bData) != SizeCapBytes+1 {
		t.Fatalf("backup size: want %d, got %d", SizeCapBytes+1, len(bData))
	}

	// New log must have the new entry only.
	nData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("new log missing: %v", err)
	}
	if !strings.Contains(string(nData), "rotate-test") {
		t.Fatalf("new log missing rotate-test entry: %q", string(nData))
	}
	if len(nData) > 1024 {
		t.Fatalf("new log too big — rotation did not truncate (got %d bytes)", len(nData))
	}
}
