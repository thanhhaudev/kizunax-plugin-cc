package cli

import (
	"fmt"
	"os"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runCancel(args []string) error {
	if len(args) < 1 {
		return xerrors.User("missing_id", "usage: kizunax cancel <job-id>", "")
	}
	id := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("state_resolve", "cannot resolve workspace state dir", err)
	}

	j, err := job.Cancel(ws, id)
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
