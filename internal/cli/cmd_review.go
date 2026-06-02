package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runReview(args []string) error {
	return runReviewWithMode(args, prompt.ModeStandard)
}

func runAdversarialReview(args []string) error {
	return runReviewWithMode(args, prompt.ModeAdversarial)
}

func runReviewWithMode(args []string, mode prompt.Mode) error {
	verbose := hasFlag(args, "--verbose")
	background := hasFlag(args, "--background")
	focus := flagValue(args, "--focus")
	providerOverride := flagValue(args, "--provider")

	target, err := parseTarget(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	if err := git.EnsureRepo(cwd); err != nil {
		return err
	}

	if background {
		return spawnBackgroundJob(cwd, mode, target, focus, providerOverride)
	}

	cfg, err := config.Load(providerOverride)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] mode=%s provider=%s model=%s base_url=%s\n",
			mode, cfg.Provider, cfg.Model, cfg.BaseURL)
		fmt.Fprintf(os.Stderr, "[verbose] target=%s\n", target.Label())
	}

	bundle, err := diff.Collect(cwd, target)
	if err != nil {
		return err
	}
	if bundle.IsEmpty() {
		fmt.Println("No changes to review for target:", target.Label())
		return nil
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] diff bytes=%d untracked=%d warnings=%d\n",
			bundle.TotalBytes, len(bundle.Untracked), len(bundle.Warnings))
	}

	pluginRoot, err := ResolvePluginRoot()
	if err != nil {
		return err
	}

	p, err := buildProvider(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, err := runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:        mode,
		Focus:       focus,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})
	if err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] tokens in=%d out=%d total=%d\n",
			result.InputTokens, result.OutputTokens, result.TotalTokens)
	}

	out := render.RenderReview(result.Review, bundle, result.TotalTokens, mode)
	fmt.Print(out)
	return nil
}

func spawnBackgroundJob(cwd string, mode prompt.Mode, target git.Target, focus, providerOverride string) error {
	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("state_resolve", "cannot resolve workspace state dir", err)
	}

	// Resolve provider name now so the worker uses the same one even if env changes.
	cfg, err := config.Load(providerOverride)
	if err != nil {
		return err
	}

	kind := job.KindReview
	if mode == prompt.ModeAdversarial {
		kind = job.KindAdversarialReview
	}

	req := job.Request{
		Mode:     mode.String(),
		Target:   target,
		Focus:    focus,
		Provider: cfg.Provider,
	}

	j, err := job.SpawnBackground(cwd, ws, kind, req)
	if err != nil {
		return err
	}

	fmt.Printf("Job %s started (kind=%s provider=%s target=%s).\n", j.ID, j.Kind, cfg.Provider, target.Label())
	fmt.Printf("Check progress: kizunax status %s\n", j.ID)
	fmt.Printf("Read result:    kizunax result %s\n", j.ID)
	return nil
}

// parseTarget reads flags --working-tree / --base / --commit / --from --to
// and optional --paths. Defaults to TargetWorkingTree if no target flag.
func parseTarget(args []string) (git.Target, error) {
	t := git.Target{Paths: splitPaths(flagValue(args, "--paths"))}

	base := flagValue(args, "--base")
	commit := flagValue(args, "--commit")
	from := flagValue(args, "--from")
	to := flagValue(args, "--to")

	chosen := 0
	if hasFlag(args, "--working-tree") {
		chosen++
	}
	if base != "" {
		chosen++
	}
	if commit != "" {
		chosen++
	}
	if from != "" || to != "" {
		chosen++
	}
	if chosen > 1 {
		return t, xerrors.User("conflict_target",
			"only one of --working-tree / --base / --commit / --from+--to may be set",
			"")
	}

	switch {
	case base != "":
		t.Kind = git.TargetBranchDiff
		t.Base = base
	case commit != "":
		t.Kind = git.TargetCommit
		t.Commit = commit
	case from != "" || to != "":
		t.Kind = git.TargetCommitRange
		t.FromSHA = from
		t.ToSHA = to
	default:
		t.Kind = git.TargetWorkingTree
	}
	return t, nil
}
