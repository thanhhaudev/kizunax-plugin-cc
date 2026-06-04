//go:build !windows

// Package job mixes live and legacy paths. Status as of v0.9.0:
//   - LEGACY: SpawnBackground + the internal-execute-job worker subcommand
//     (no callers from review since v0.9 — async is delegated to Claude Code's
//     Bash(run_in_background:true). Kept for any future task-delegation
//     command. Safe to remove if no longer needed by end of v0.10.)
//   - LIVE: Cancel, SweepOrphans. Both invoked by /kizunax:cancel and
//     /kizunax:status respectively. Removing these would break v0.9 status
//     UI for legacy v0.8 running jobs.

package job

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// SpawnBackground writes an initial job record and starts a detached worker
// child that will call back into this binary with `internal-execute-job <id>`.
func SpawnBackground(cwd string, ws state.WorkspaceDir, kind Kind, req Request) (Job, error) {
	id := NewID()
	now := time.Now().UTC()
	j := Job{
		ID:        id,
		Kind:      kind,
		Status:    StatusRunning,
		CreatedAt: now,
		StartedAt: now,
		Request:   req,
		LogPath:   ws.LogPath(id),
	}

	if err := Save(ws, j); err != nil {
		return j, xerrors.Internal("save_job", "cannot save initial job record", err)
	}

	logFile, err := os.OpenFile(j.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return j, xerrors.Background("open_log",
			fmt.Sprintf("cannot open log file %s", j.LogPath), "", err)
	}

	self, err := os.Executable()
	if err != nil {
		logFile.Close()
		return j, xerrors.Internal("self_exe", "cannot resolve binary path", err)
	}

	cmd := exec.Command(self, "internal-execute-job", id)
	cmd.Dir = cwd
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // new process group so kill -pid -- group works
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		j.Status = StatusFailed
		j.Error = fmt.Sprintf("spawn worker: %v", err)
		completed := time.Now().UTC()
		j.CompletedAt = &completed
		_ = Save(ws, j)
		return j, xerrors.Background("spawn", "cannot spawn worker", "", err)
	}

	// Parent doesn't wait — child runs detached.
	j.PID = cmd.Process.Pid
	if err := Save(ws, j); err != nil {
		// Already spawned; best-effort update only.
		fmt.Fprintf(os.Stderr, "warning: cannot save job pid: %v\n", err)
	}

	// Detach from child without leaving zombie: caller closes logFile handle in parent;
	// child inherited dup'd fd already.
	logFile.Close()
	return j, nil
}

// Cancel marks a running job cancelled and SIGTERMs its process group.
func Cancel(ws state.WorkspaceDir, id string) (Job, error) {
	j, err := Load(ws, id)
	if err != nil {
		return j, xerrors.User("job_not_found",
			fmt.Sprintf("no job with id %s", id), "")
	}
	if j.Status != StatusRunning {
		return j, xerrors.User("not_running",
			fmt.Sprintf("job %s is already %s", id, j.Status), "")
	}
	if j.PID > 0 {
		_ = syscall.Kill(-j.PID, syscall.SIGTERM)
	}
	j.Status = StatusCancelled
	completed := time.Now().UTC()
	j.CompletedAt = &completed
	if err := Save(ws, j); err != nil {
		return j, xerrors.Internal("save_cancel", "cannot save cancelled state", err)
	}
	return j, nil
}

// SweepOrphans marks jobs whose worker PID no longer exists as failed.
// Called lazily on /status to reconcile crashes.
func SweepOrphans(ws state.WorkspaceDir) {
	jobs, _ := List(ws)
	for _, j := range jobs {
		if j.Status != StatusRunning || j.PID <= 0 {
			continue
		}
		proc, err := os.FindProcess(j.PID)
		if err != nil {
			markOrphan(ws, j, "process not found")
			continue
		}
		// signal-0 is alive-check on Unix.
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			markOrphan(ws, j, fmt.Sprintf("process disappeared: %v", err))
		}
	}
}

func markOrphan(ws state.WorkspaceDir, j Job, reason string) {
	j.Status = StatusFailed
	j.Error = reason
	completed := time.Now().UTC()
	j.CompletedAt = &completed
	_ = Save(ws, j)
}
