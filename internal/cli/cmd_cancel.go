package cli

import (
	"errors"
	"fmt"
	"os"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runCancel(args []string) error {
	// Parse flags + positional id.
	all := false
	var id string
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		default:
			if !startsWithFlag(a) && id == "" {
				id = a
			}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("state_resolve", "cannot resolve workspace state dir", err)
	}

	// Default-scope to the current CC session; --all bypasses.
	var pool []job.Job
	if all {
		pool, err = job.List(ws)
	} else {
		pool, err = job.ListBySession(ws, CurrentSessionID())
	}
	if err != nil {
		return xerrors.Internal("list_jobs", "cannot list jobs", err)
	}

	// Only running jobs are cancellable.
	running := pool[:0]
	for _, j := range pool {
		if j.Status == job.StatusRunning {
			running = append(running, j)
		}
	}

	target, matchErr := job.MatchByPrefix(running, id)
	if matchErr != nil {
		switch {
		case errors.Is(matchErr, job.ErrAmbiguousJobID):
			return xerrors.User("cancel_ambiguous", matchErr.Error(), "use a longer id prefix")
		case errors.Is(matchErr, job.ErrJobNotFound):
			ref := id
			if ref == "" {
				ref = "(none)"
			}
			return xerrors.User("cancel_lookup",
				fmt.Sprintf("no active job matching %q", ref),
				"Pass a job id or use /kizunax:status to list active jobs.")
		default:
			return xerrors.Internal("job_lookup", "cannot match job id", matchErr)
		}
	}

	// Reuse existing cancel logic: signal worker process group and persist state.
	j, err := job.Cancel(ws, target.ID)
	if err != nil {
		return err
	}
	if j.PID > 0 {
		fmt.Printf("Job %s cancelled (SIGTERM sent to worker PID %d).\n", j.ID, j.PID)
	} else {
		fmt.Printf("Job %s cancelled.\n", j.ID)
	}
	return nil
}
