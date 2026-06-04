package hooks

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

func TestDeleteOldLogs_RemovesAged(t *testing.T) {
	ws := makeWS(t)
	old := filepath.Join(ws.JobsDir(), "old.log")
	recent := filepath.Join(ws.JobsDir(), "recent.log")
	notLog := filepath.Join(ws.JobsDir(), "keep.json")

	for _, p := range []string{old, recent, notLog} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(old, tenDaysAgo, tenDaysAgo); err != nil {
		t.Fatal(err)
	}

	deleted := DeleteOldLogs(ws, 7*24*time.Hour)
	if deleted != 1 {
		t.Errorf("deleted: got %d want 1", deleted)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old log not removed")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("recent log removed (should be kept): %v", err)
	}
	if _, err := os.Stat(notLog); err != nil {
		t.Errorf(".json file removed (should be kept): %v", err)
	}
}

func TestDeleteOldLogs_MissingJobsDirNoError(t *testing.T) {
	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	deleted := DeleteOldLogs(ws, 7*24*time.Hour)
	if deleted != 0 {
		t.Errorf("deleted: got %d want 0", deleted)
	}
}
