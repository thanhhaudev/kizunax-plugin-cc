package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ExpansionState persists the per-workspace toggle for v1.1.0 bundle
// expansion strategies. Each bool maps 1:1 to llmreviewkit's
// engine.Config.Expand* fields. Default zero-value is all-off, which
// produces v0.15.0-identical behavior.
//
// Env vars KIZUNAX_DISABLE_EXPAND and KIZUNAX_EXPAND still take
// precedence (for one-shot kill / testing); this file is the persistent
// baseline set via `/kizunax:expansion enable|disable|set|reset`.
type ExpansionState struct {
	Callers  bool `json:"callers"`
	TypeDefs bool `json:"typedefs"`
	Tests    bool `json:"tests"`
}

func (w WorkspaceDir) ExpansionPath() string {
	return filepath.Join(w.Root, "expansion.json")
}

// LoadExpansion returns the expansion state. Missing file or corrupt
// JSON returns zero-value (all false) without error — same forgiveness
// pattern as LoadUseIndex.
func LoadExpansion(ws WorkspaceDir) (ExpansionState, error) {
	var s ExpansionState
	data, err := os.ReadFile(ws.ExpansionPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return ExpansionState{}, nil
	}
	return s, nil
}

func SaveExpansion(ws WorkspaceDir, s ExpansionState) error {
	if err := os.MkdirAll(filepath.Dir(ws.ExpansionPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(ws.ExpansionPath(), data, 0o600)
}

// DeleteExpansion removes the state file. Missing file is not an error.
func DeleteExpansion(ws WorkspaceDir) error {
	err := os.Remove(ws.ExpansionPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
