package cli

import (
	"fmt"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runResult(args []string) error {
	if len(args) < 1 {
		return xerrors.User("missing_id", "usage: kizunax result <job-id>", "")
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

	j, err := job.Load(ws, id)
	if err != nil {
		return xerrors.User("job_not_found",
			fmt.Sprintf("no job with id %s", id),
			"use `kizunax status` to list available jobs")
	}

	switch j.Status {
	case job.StatusRunning:
		fmt.Printf("Job %s is still running. Try `kizunax status %s`.\n", id, id)
		return nil
	case job.StatusFailed:
		fmt.Printf("Job %s failed:\n  %s\n\nLog: %s\n", id, j.Error, j.LogPath)
		return nil
	case job.StatusCancelled:
		fmt.Printf("Job %s was cancelled.\n", id)
		return nil
	}

	// completed
	if j.Result == nil {
		fmt.Println("Job completed but no result is stored.")
		return nil
	}

	mode := prompt.ModeStandard
	if j.Kind == job.KindAdversarialReview {
		mode = prompt.ModeAdversarial
	}

	// Reconstruct a minimal bundle for rendering (we don't re-collect diff).
	bundle := diff.Bundle{
		TargetLabel: j.Request.Target.Label(),
		Warnings:    j.Warnings,
	}

	totalTokens := 0
	if j.Tokens != nil {
		totalTokens = j.Tokens.Total
	}
	fmt.Print(render.RenderReview(*j.Result, bundle, totalTokens, mode))

	// Bound the footer lookup to the exact key the worker used (persisted at
	// run time). Avoids calling config.Load which rotates round-robin.
	appendUsageFooterByHash(os.Stdout, ws, j.Request.KeyHash, j.Request.KeyMask)
	return nil
}
