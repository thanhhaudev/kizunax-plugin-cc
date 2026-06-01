package cli

import (
	"fmt"
	"os"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
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

	// If a job id is given, render single-job detail.
	if len(args) > 0 && !startsWithFlag(args[0]) {
		id := args[0]
		j, err := job.Load(ws, id)
		if err != nil {
			return xerrors.User("job_not_found",
				fmt.Sprintf("no job with id %s", id), "use `kizunax status` (no args) to list")
		}
		fmt.Print(render.RenderJobDetail(j))
		return nil
	}

	jobs, err := job.List(ws)
	if err != nil {
		return xerrors.Internal("list_jobs", "cannot list jobs", err)
	}
	fmt.Print(render.RenderStatusList(jobs))
	return nil
}

func startsWithFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
