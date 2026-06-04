package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/thanhhaudev/llmreviewkit/diff"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runResult(args []string) error {
	// Parse positional id (optional). Empty ref → newest job.
	var id string
	for _, a := range args {
		if !startsWithFlag(a) && id == "" {
			id = a
			break
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

	// Result lookup intentionally spans ALL sessions so users can inspect
	// history regardless of which CC session originated the job.
	all, err := job.List(ws)
	if err != nil {
		return xerrors.Internal("list_jobs", "cannot list jobs", err)
	}

	j, matchErr := job.MatchByPrefix(all, id)
	if matchErr != nil {
		switch {
		case errors.Is(matchErr, job.ErrAmbiguousJobID):
			return xerrors.User("job_ambiguous", matchErr.Error(), "use a longer id prefix")
		case errors.Is(matchErr, job.ErrJobNotFound):
			ref := id
			if ref == "" {
				ref = "(latest)"
			}
			return xerrors.User("job_not_found",
				fmt.Sprintf("no job with id or prefix %q", ref),
				"Use /kizunax:status --all to list.")
		default:
			return xerrors.Internal("job_lookup", "cannot match job id", matchErr)
		}
	}

	switch j.Status {
	case job.StatusRunning:
		fmt.Printf("Job %s is still running. Try `kizunax status %s`.\n", j.ID, j.ID)
		return nil
	case job.StatusFailed:
		fmt.Printf("Job %s failed:\n  %s\n\nLog: %s\n", j.ID, j.Error, j.LogPath)
		return nil
	case job.StatusCancelled:
		fmt.Printf("Job %s was cancelled.\n", j.ID)
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
