package hooks

import (
	"fmt"
	"io"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// SessionCleanup performs idempotent housekeeping safe to run on either
// SessionStart or SessionEnd: marks running jobs whose PID is gone as
// failed (via job.SweepOrphans), then deletes *.log files older than 7
// days. Always returns 0 — the hook must not block CC.
func SessionCleanup(in io.Reader, out, errOut io.Writer, ws state.WorkspaceDir) int {
	defer recoverSilent(errOut, "session-cleanup")

	job.SweepOrphans(ws)
	deleted := DeleteOldLogs(ws, 7*24*time.Hour)
	fmt.Fprintf(errOut, "[kizunax-hook session-cleanup] swept orphans, deleted %d old log files\n", deleted)
	return 0
}

// recoverSilent is the deferred recovery used by every hook entry. Any
// panic gets logged to stderr and the hook returns its caller value
// (already set before the panic) — usually 0, so CC never blocks.
func recoverSilent(errOut io.Writer, name string) {
	if r := recover(); r != nil {
		fmt.Fprintf(errOut, "[kizunax-hook %s] panic recovered: %v\n", name, r)
	}
}
