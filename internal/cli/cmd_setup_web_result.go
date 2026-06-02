package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
)

// setupWebOutcome is the terminal status of a completed setup-web run.
type setupWebOutcome string

const (
	setupWebSuccess   setupWebOutcome = "success"
	setupWebTimeout   setupWebOutcome = "timeout"
	setupWebCancelled setupWebOutcome = "cancelled"
)

// setupWebResult is the last-completed-run record.
type setupWebResult struct {
	Outcome     setupWebOutcome `json:"outcome"`
	Message     string          `json:"message"`
	CompletedAt time.Time       `json:"completed_at"`
	ConfigPath  string          `json:"config_path,omitempty"`
}

// setupWebResultPath returns the absolute path to the result file.
func setupWebResultPath() (string, error) {
	configPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), ".setup-web.result.json"), nil
}

// writeSetupWebResult writes the result atomically. Errors are returned to the
// caller but treated as non-fatal by the worker (the user can still see
// success via the browser tab; this file is best-effort).
func writeSetupWebResult(r setupWebResult) error {
	path, err := setupWebResultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadSetupWebResult reads the last-completed result. Returns os.ErrNotExist
// if the file is absent.
func loadSetupWebResult() (setupWebResult, error) {
	var r setupWebResult
	path, err := setupWebResultPath()
	if err != nil {
		return r, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	return r, nil
}
