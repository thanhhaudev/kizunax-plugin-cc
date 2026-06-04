package cli

import (
	"errors"
	"fmt"
	"os"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/pkg/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runStatus(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("state_resolve", "cannot resolve workspace state dir", err)
	}

	// Sweep orphans (running jobs whose worker died) before listing.
	job.SweepOrphans(ws)

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

	// Choose listing scope. --all bypasses the per-session filter.
	var jobs []job.Job
	if all {
		jobs, err = job.List(ws)
	} else {
		jobs, err = job.ListBySession(ws, CurrentSessionID())
	}
	if err != nil {
		return xerrors.Internal("list_jobs", "cannot list jobs", err)
	}

	// If a job id (or prefix) is given, render single-job detail.
	if id != "" {
		// Search across *all* jobs in the workspace so prefix lookup works even
		// when the id belongs to a different session.
		searchSet := jobs
		if !all {
			searchSet, err = job.List(ws)
			if err != nil {
				return xerrors.Internal("list_jobs", "cannot list jobs", err)
			}
		}
		j, matchErr := job.MatchByPrefix(searchSet, id)
		if matchErr != nil {
			switch {
			case errors.Is(matchErr, job.ErrAmbiguousJobID):
				return xerrors.User("job_ambiguous", matchErr.Error(), "use a longer id prefix")
			case errors.Is(matchErr, job.ErrJobNotFound):
				return xerrors.User("job_not_found",
					fmt.Sprintf("no job with id or prefix %q", id),
					"use `kizunax status` (no args) to list")
			default:
				return xerrors.Internal("job_lookup", "cannot match job id", matchErr)
			}
		}
		fmt.Print(render.RenderJobDetail(j))
		return nil
	}

	fmt.Print(render.RenderStatusList(jobs))
	return nil
}

func startsWithFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
