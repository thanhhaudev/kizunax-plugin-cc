package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

func runReview(args []string) error {
	return runReviewWithMode(args, prompt.ModeStandard)
}

func runAdversarialReview(args []string) error {
	return runReviewWithMode(args, prompt.ModeAdversarial)
}

func runReviewWithMode(args []string, mode prompt.Mode) error {
	verbose := hasFlag(args, "--verbose")
	quiet := hasFlag(args, "--quiet")
	// --background is accepted for backward compatibility but is a no-op since
	// v0.9. Async execution is delegated to Claude Code's
	// Bash(run_in_background:true) at the slash-command layer.
	_ = hasFlag(args, "--background")
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
	start := time.Now()
	result, err := runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:        mode,
		Focus:       focus,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})
	dur := time.Since(start)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] tokens in=%d out=%d total=%d\n",
			result.InputTokens, result.OutputTokens, result.TotalTokens)
		fmt.Fprintf(os.Stderr, "[verbose] duration=%dms model=%s\n",
			dur.Milliseconds(), cfg.Model)
	}

	out := render.RenderReview(result.Review, bundle, result.TotalTokens, mode)
	fmt.Print(out)

	// Sync refresh BEFORE footer so the cache reflects this review's quota
	// usage. RefreshAsync alone would race process exit and skip the HTTP.
	if ws, wsErr := state.Resolve(cwd); wsErr == nil {
		if base, baseErr := usage.DeriveBase(cfg.BaseURL); baseErr == nil {
			usage.RefreshAndWait(base, cfg.APIKey, ws, 6*time.Second)
		}
		appendUsageFooterIfNotQuiet(os.Stdout, quiet, ws, cfg.APIKey)
	}
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
