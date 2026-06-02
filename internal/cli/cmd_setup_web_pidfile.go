package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
)

// setupWebPIDPath returns the absolute path to the setup-web PID file
// (~/.kizunax/.setup-web.pid).
func setupWebPIDPath() (string, error) {
	configPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), ".setup-web.pid"), nil
}

// writeSetupWebPID writes pid to ~/.kizunax/.setup-web.pid (0600), creating
// the directory if needed.
func writeSetupWebPID(pid int) error {
	path, err := setupWebPIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// removeSetupWebPID removes the PID file. Returns nil if it was already absent.
func removeSetupWebPID() error {
	path, err := setupWebPIDPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// killOldSetupWebWorker reads the PID file (if present), sends SIGTERM to that
// pid if the process is alive, and waits briefly for it to exit. Always returns
// nil — kill failures are non-fatal (e.g. the old worker is already dead).
func killOldSetupWebWorker() error {
	path, err := setupWebPIDPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		_ = os.Remove(path)
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(path)
		return nil
	}
	// Signal 0 is an alive-check on Unix.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process is gone.
		_ = os.Remove(path)
		return nil
	}
	// Send SIGTERM; ignore errors (race against natural exit).
	_ = proc.Signal(syscall.SIGTERM)
	// Brief wait so the new listener doesn't collide with leftover state.
	// We don't WaitForExit — process is detached and we don't own it.
	for i := 0; i < 20; i++ {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.Remove(path)
	return nil
}

