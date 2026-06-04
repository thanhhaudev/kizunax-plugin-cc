package state

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/statedir"
)

// WorkspaceDir is the kizunax-flavored per-workspace state directory.
// It embeds pkg/statedir.WorkspaceDir so the generic surface (Root,
// Resolve via package fn) carries over, and adds kizunax-specific
// schemas (jobs/, stop-gate.json, use_index.json).
type WorkspaceDir struct {
	statedir.WorkspaceDir
}

// NewWorkspaceDir constructs a WorkspaceDir from a raw root path. Useful
// in tests where the directory already exists and Resolve's mkdir logic
// is unnecessary.
func NewWorkspaceDir(root string) WorkspaceDir {
	return WorkspaceDir{WorkspaceDir: statedir.WorkspaceDir{Root: root}}
}

// Resolve derives a per-workspace state dir under ~/.kizunax/state/ and
// ensures the kizunax-required jobs/ subdir exists.
func Resolve(cwd string) (WorkspaceDir, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return WorkspaceDir{}, err
	}
	base := filepath.Join(home, ".kizunax", "state")
	inner, err := statedir.Resolve(base, cwd)
	if err != nil {
		return WorkspaceDir{}, err
	}
	if err := os.MkdirAll(filepath.Join(inner.Root, "jobs"), 0o700); err != nil {
		return WorkspaceDir{}, err
	}
	return WorkspaceDir{WorkspaceDir: inner}, nil
}

func (w WorkspaceDir) JobsDir() string         { return filepath.Join(w.Root, "jobs") }
func (w WorkspaceDir) JobPath(id string) string { return filepath.Join(w.JobsDir(), id+".json") }
func (w WorkspaceDir) LogPath(id string) string { return filepath.Join(w.JobsDir(), id+".log") }

// ListJobIDs returns IDs of all job records in the workspace, sorted lexically
// (which equals chronological order because IDs are time-prefixed).
func (w WorkspaceDir) ListJobIDs() ([]string, error) {
	entries, err := os.ReadDir(w.JobsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	return ids, nil
}

// WriteAtomic is preserved as a thin back-compat alias so existing callers
// (internal/state/stop_gate.go, internal/state/use_index.go, internal/job/*)
// don't need import rewrites. The canonical implementation lives in pkg/statedir.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	return statedir.WriteAtomic(path, data, perm)
}
