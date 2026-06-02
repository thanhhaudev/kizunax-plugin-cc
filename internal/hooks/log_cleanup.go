package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// DeleteOldLogs removes *.log files in ws.JobsDir() whose mtime is older
// than maxAge. Returns the count of deleted files. Missing directory is
// not an error (returns 0). I/O failures on individual files are silently
// skipped.
func DeleteOldLogs(ws state.WorkspaceDir, maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	entries, err := os.ReadDir(ws.JobsDir())
	if err != nil {
		return 0
	}
	deleted := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(ws.JobsDir(), e.Name())); err == nil {
				deleted++
			}
		}
	}
	return deleted
}
