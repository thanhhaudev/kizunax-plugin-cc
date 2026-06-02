package job

import (
	"os"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func mkdirAll(p string) error {
	return os.MkdirAll(p, 0o700)
}

// tempWorkspace returns a WorkspaceDir rooted at t.TempDir() with the jobs
// directory pre-created. Mirrors the inline pattern used elsewhere in this
// package's tests.
func tempWorkspace(t *testing.T) state.WorkspaceDir {
	t.Helper()
	ws := state.WorkspaceDir{Root: t.TempDir()}
	if err := mkdirAll(ws.JobsDir()); err != nil {
		t.Fatalf("setup jobs dir: %v", err)
	}
	return ws
}
