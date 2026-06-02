package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
)

// setupWebState describes the on-disk state of a currently running setup-web
// worker. Stored as JSON at ~/.kizunax/.setup-web.pid (mode 0600).
type setupWebState struct {
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	IdleDeadline time.Time `json:"idle_deadline"`
}

// setupWebStatePath returns the absolute path to the setup-web state file.
// The filename is `.setup-web.pid` for migration compatibility with v0.6.4
// (which wrote a plain integer there); the new format is JSON.
func setupWebStatePath() (string, error) {
	configPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), ".setup-web.pid"), nil
}

// writeSetupWebState writes the JSON state to ~/.kizunax/.setup-web.pid (0600),
// atomically via tmp + rename.
func writeSetupWebState(s setupWebState) error {
	path, err := setupWebStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadSetupWebState reads the state file. Tolerant of v0.6.4 format: if JSON
// parse fails, falls back to parsing the file body as a plain integer PID
// (zero-value StartedAt / IdleDeadline). Returns os.ErrNotExist if the file
// is absent.
func loadSetupWebState() (setupWebState, error) {
	var s setupWebState
	path, err := setupWebStatePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err == nil && s.PID > 0 {
		return s, nil
	}
	pid, perr := strconv.Atoi(strings.TrimSpace(string(data)))
	if perr != nil || pid <= 0 {
		return setupWebState{}, perr
	}
	return setupWebState{PID: pid}, nil
}

// removeSetupWebState removes the state file. Returns nil if it was already absent.
func removeSetupWebState() error {
	path, err := setupWebStatePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// killOldSetupWebWorker reads the state file, SIGTERMs the recorded PID if it
// is alive, then removes the state file. Always returns nil — kill failures
// are non-fatal (e.g. the old worker is already dead).
func killOldSetupWebWorker() error {
	s, err := loadSetupWebState()
	if err != nil || s.PID <= 0 {
		_ = removeSetupWebState()
		return nil
	}
	proc, err := os.FindProcess(s.PID)
	if err != nil {
		_ = removeSetupWebState()
		return nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		_ = removeSetupWebState()
		return nil
	}
	_ = proc.Signal(syscall.SIGTERM)
	for i := 0; i < 20; i++ {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = removeSetupWebState()
	return nil
}
