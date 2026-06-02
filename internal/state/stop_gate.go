package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type StopGateState struct {
	Enabled            bool           `json:"enabled"`
	LastHash           []byte         `json:"lastHash,omitempty"`
	LastRunAt          time.Time      `json:"lastRunAt,omitempty"`
	LastVerdictHadHigh bool           `json:"lastVerdictHadHigh,omitempty"`
	LastResult         *CachedVerdict `json:"lastResult,omitempty"`
}

type CachedVerdict struct {
	Findings []CachedFinding `json:"findings"`
}

type CachedFinding struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Title    string `json:"title"`
}

func (w WorkspaceDir) StopGatePath() string {
	return filepath.Join(w.Root, "stop-gate.json")
}

// LoadStopGate returns the stop-gate state. Missing file or corrupt JSON
// returns a zero-value state (Enabled=false) without error.
func LoadStopGate(ws WorkspaceDir) (StopGateState, error) {
	var s StopGateState
	data, err := os.ReadFile(ws.StopGatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return StopGateState{}, nil
	}
	return s, nil
}

func SaveStopGate(ws WorkspaceDir, s StopGateState) error {
	if err := os.MkdirAll(filepath.Dir(ws.StopGatePath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(ws.StopGatePath(), data, 0o600)
}
