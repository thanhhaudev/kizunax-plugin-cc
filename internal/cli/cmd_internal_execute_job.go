package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

// runInternalExecuteJob is the worker entry point spawned by SpawnBackground.
// It is NOT shown in --help and is not meant for user invocation.
func runInternalExecuteJob(args []string) error {
	if len(args) < 1 {
		return xerrors.Internal("missing_id", "internal-execute-job requires <id>", nil)
	}
	id := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	ws, err := state.Resolve(cwd)
	if err != nil {
		return err
	}

	j, err := job.Load(ws, id)
	if err != nil {
		return xerrors.Internal("load_job", fmt.Sprintf("cannot load job %s", id), err)
	}

	fmt.Fprintf(os.Stdout, "[worker] %s starting kind=%s target=%s\n",
		j.ID, j.Kind, j.Request.Target.Label())

	j.StartedAt = time.Now().UTC()

	if err := executeJobBody(cwd, ws, &j); err != nil {
		j.Status = job.StatusFailed
		j.Error = err.Error()
		completed := time.Now().UTC()
		j.CompletedAt = &completed
		j.DurationMs = completed.Sub(j.StartedAt).Milliseconds()
		_ = job.Save(ws, j)
		fmt.Fprintf(os.Stderr, "[worker] %s failed: %v\n", j.ID, err)
		return err
	}

	j.Status = job.StatusCompleted
	completed := time.Now().UTC()
	j.CompletedAt = &completed
	j.DurationMs = completed.Sub(j.StartedAt).Milliseconds()
	if err := job.Save(ws, j); err != nil {
		fmt.Fprintf(os.Stderr, "[worker] cannot save final state: %v\n", err)
	}
	fmt.Fprintf(os.Stdout, "[worker] %s completed\n", j.ID)
	return nil
}

func executeJobBody(cwd string, ws state.WorkspaceDir, j *job.Job) error {
	cfg, err := config.Load(j.Request.Provider)
	if err != nil {
		return err
	}
	// Pin the picked key into the job record so `kizunax result` can read the
	// usage cache for this exact key — config.Load rotates round-robin and a
	// later Load may return a different key.
	j.Request.KeyHash = usage.HashKey(cfg.APIKey)
	j.Request.KeyMask = usage.MaskKey(cfg.APIKey)
	// Pin the model used by this worker so the on-disk record reflects reality
	// even if config rotates. New jobs (spawned with Model set) keep their
	// value; backfill only for legacy jobs missing the field.
	if j.Request.Model == "" {
		j.Request.Model = cfg.Model
	}

	bundle, err := diff.Collect(cwd, j.Request.Target)
	if err != nil {
		return err
	}
	if bundle.IsEmpty() {
		return xerrors.Diff("empty_diff", "no changes to review for target", "")
	}

	pluginRoot, err := ResolvePluginRoot()
	if err != nil {
		return err
	}

	p, err := buildProvider(cfg)
	if err != nil {
		return err
	}

	mode := prompt.ModeStandard
	if j.Kind == job.KindAdversarialReview {
		mode = prompt.ModeAdversarial
	}

	ctx := context.Background()
	result, err := runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:          mode,
		Focus:         j.Request.Focus,
		Model:         cfg.Model,
		Temperature:   cfg.Temperature,
		MaxTokens:     cfg.MaxTokens,
		WorkspaceRoot: cwd, // v0.12: enable enrichment when background worker re-runs
	})
	if err != nil {
		return err
	}

	j.Result = &result.Review
	j.Warnings = bundle.Warnings
	j.Tokens = &job.TokenUsage{
		Input:  result.InputTokens,
		Output: result.OutputTokens,
		Total:  result.TotalTokens,
	}

	// Also append rendered markdown to the log file for convenience.
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "=== RENDERED REVIEW ===")
	fmt.Fprint(os.Stdout, render.RenderReview(result.Review, bundle, result.TotalTokens, mode))

	// Refresh usage cache so `kizunax result` sees a low-quota footer if needed.
	// Synchronous-bounded within the worker process; the parent already exited.
	if base, err := usage.DeriveBase(cfg.BaseURL); err == nil {
		usage.RefreshAndWait(base, cfg.APIKey, ws, 6*time.Second)
	}

	return nil
}
