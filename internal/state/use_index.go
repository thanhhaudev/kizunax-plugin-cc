package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UseIndexState persists the per-workspace toggle for v0.13 index-backed
// resolver. The env var KIZUNAX_USE_INDEX still takes precedence (for
// one-shot testing); this file is the persistent baseline set via
// `kizunax index enable` / `kizunax index disable`.
type UseIndexState struct {
	Enabled bool `json:"enabled"`
}

func (w WorkspaceDir) UseIndexPath() string {
	return filepath.Join(w.Root, "use_index.json")
}

// LoadUseIndex returns the use-index state. Missing file or corrupt JSON
// returns zero-value (Enabled=false) without error.
func LoadUseIndex(ws WorkspaceDir) (UseIndexState, error) {
	var s UseIndexState
	data, err := os.ReadFile(ws.UseIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return UseIndexState{}, nil
	}
	return s, nil
}

func SaveUseIndex(ws WorkspaceDir, s UseIndexState) error {
	if err := os.MkdirAll(filepath.Dir(ws.UseIndexPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(ws.UseIndexPath(), data, 0o600)
}
