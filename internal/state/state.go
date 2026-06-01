package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceDir struct {
	Root string
}

// Resolve derives a per-workspace state dir under ~/.kizunax/state/.
// Dir name combines basename + sha256 prefix(abspath) so two repos with the
// same basename at different paths get isolated state.
func Resolve(cwd string) (WorkspaceDir, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return WorkspaceDir{}, err
	}
	base := filepath.Base(abs)
	h := sha256.Sum256([]byte(abs))
	name := fmt.Sprintf("%s-%s", base, hex.EncodeToString(h[:8]))

	home, err := os.UserHomeDir()
	if err != nil {
		return WorkspaceDir{}, err
	}
	root := filepath.Join(home, ".kizunax", "state", name)
	if err := os.MkdirAll(filepath.Join(root, "jobs"), 0o700); err != nil {
		return WorkspaceDir{}, err
	}
	return WorkspaceDir{Root: root}, nil
}

func (w WorkspaceDir) JobsDir() string { return filepath.Join(w.Root, "jobs") }

func (w WorkspaceDir) JobPath(id string) string {
	return filepath.Join(w.JobsDir(), id+".json")
}

func (w WorkspaceDir) LogPath(id string) string {
	return filepath.Join(w.JobsDir(), id+".log")
}

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

// WriteAtomic writes data to path via tmp+rename for crash safety.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
