//go:build !windows

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/fanout"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
	"github.com/thanhhaudev/llmreviewkit/diff"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/git"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/render"
	"github.com/thanhhaudev/llmreviewkit/schema"
)

type runFanoutArgs struct {
	cwd          string
	target       git.Target
	bundle       diff.Bundle
	cfg          config.Config
	mode         prompt.Mode
	focus        string
	verbose      bool
	quiet        bool
	originalArgs []string
	wsDir        state.WorkspaceDir
}

func runFanoutReview(ctx context.Context, a runFanoutArgs) error {
	// 1. List changed files via git.
	files, err := listChangedFiles(a.cwd, a.target)
	if err != nil {
		return xerrors.Internal("fanout_list_files", "could not list changed files", err)
	}
	if len(files) == 0 {
		fmt.Println("No changes to review for target:", a.target.Label())
		return nil
	}

	// 2. Group into buckets.
	buckets := fanout.Group(files)
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[verbose] fanout: %d buckets\n", len(buckets))
		for _, b := range buckets {
			fmt.Fprintf(os.Stderr, "[verbose]   %s (%d files)\n", b.Prefix, len(b.Files))
		}
	}

	// 3. If only 1 bucket survived, fan-out isn't useful — fall back to single review.
	if len(buckets) <= 1 {
		if a.verbose {
			fmt.Fprintln(os.Stderr, "[verbose] fanout: <=1 bucket — falling back to single review")
		}
		// Strip --strategy so the recursive call uses single mode.
		args := filterStrategyFlag(a.originalArgs)
		args = append(args, "--strategy=single")
		return runReviewWithMode(args, a.mode)
	}

	// 4. Resolve self binary path for worker spawning.
	selfPath, err := os.Executable()
	if err != nil {
		return xerrors.Internal("fanout_self_path", "could not resolve self executable", err)
	}

	subcommand := "review"
	if a.mode == prompt.ModeAdversarial {
		subcommand = "adversarial-review"
	}
	workerArgs := buildWorkerArgs(a.originalArgs, a.target)

	fmt.Fprintf(os.Stderr, "[info] fan-out: spawning %d workers via %s (batch up to 4)\n",
		len(buckets), a.cfg.Provider)

	// 5. Spawn workers in batches of 4.
	results, err := fanout.Run(ctx, buckets, fanout.SpawnOptions{
		BinaryPath:       selfPath,
		Subcommand:       subcommand,
		BaseArgs:         workerArgs,
		Concurrency:      4,
		PerBucketTimeout: 15 * time.Minute,
		WorkingDir:       a.cwd,
		ProgressFn: func(done, total int, prefix string) {
			fmt.Fprintf(os.Stderr, "[info] fan-out: %d/%d buckets done (%s)\n", done, total, prefix)
		},
	})
	if err != nil {
		return xerrors.Internal("fanout_run", "fanout dispatch failed", err)
	}

	// 6. Decode each worker's JSON stdout + collect reviews.
	var reviews []fanout.BucketReview
	var workerErrs []string
	for _, r := range results {
		if r.Err != nil {
			workerErrs = append(workerErrs, fmt.Sprintf("%s: %v", r.Bucket.Prefix, r.Err))
			continue
		}
		var rv schema.ReviewResult
		if err := json.Unmarshal([]byte(r.Stdout), &rv); err != nil {
			workerErrs = append(workerErrs, fmt.Sprintf("%s: invalid worker JSON (%v); stderr=%s",
				r.Bucket.Prefix, err, truncate(r.Stderr, 200)))
			continue
		}
		reviews = append(reviews, fanout.BucketReview{Bucket: r.Bucket, Result: rv})
	}
	for _, e := range workerErrs {
		fmt.Fprintf(os.Stderr, "[warn] fanout bucket failed: %s\n", e)
	}
	if len(reviews) == 0 {
		return xerrors.Internal("fanout_all_failed", "no fan-out worker succeeded", nil)
	}

	// 7. Merge results.
	merged := fanout.Merge(reviews, fanout.MergeOptions{
		AnnotateFindings: false,
		BuildTLDR:        true,
	})

	// 8. Render once.
	out := render.RenderReview(merged, a.bundle, 0, a.mode)
	fmt.Print(out)

	// 9. Post-flight: usage refresh + footer (once, for the whole fan-out).
	if ws, wsErr := state.Resolve(a.cwd); wsErr == nil {
		if base, baseErr := usage.DeriveBase(a.cfg.BaseURL); baseErr == nil {
			usage.RefreshAndWait(base, a.cfg.APIKey, ws, 6*time.Second)
		}
		appendUsageFooterIfNotQuiet(os.Stdout, a.quiet, ws, a.cfg.APIKey)
	}
	return nil
}

// listChangedFiles uses git to enumerate changed files for the given target.
// Mirrors the baseline-mismatch logic in cmd_review.go.
func listChangedFiles(cwd string, target git.Target) ([]string, error) {
	var cmd *exec.Cmd
	switch target.Kind {
	case git.TargetBranchDiff:
		cmd = exec.Command("git", "diff", "--name-only", target.Base+"...HEAD")
	case git.TargetCommit:
		cmd = exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", target.Commit)
	case git.TargetCommitRange:
		cmd = exec.Command("git", "diff", "--name-only", target.FromSHA+".."+target.ToSHA)
	default:
		// TargetWorkingTree
		cmd = exec.Command("git", "diff", "--name-only", "HEAD")
	}
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, l := range lines {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// buildWorkerArgs filters the original CLI args to keep only flags that should
// propagate to workers and always appends --json. Strips --strategy (workers run
// single mode implicitly via --json guard) and relies on fanout.buildArgs to add
// --paths per bucket.
func buildWorkerArgs(orig []string, _ git.Target) []string {
	var out []string
	skipNext := false
	for _, a := range orig {
		if skipNext {
			skipNext = false
			continue
		}
		// Strip --strategy and its value.
		if a == "--strategy" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "--strategy=") {
			continue
		}
		// Strip --paths (fanout.buildArgs adds per-bucket paths).
		if a == "--paths" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "--paths=") {
			continue
		}
		out = append(out, a)
	}
	// Workers always get --json so they emit schema.ReviewResult to stdout.
	out = append(out, "--json")
	return out
}

// truncate clips a string to n bytes, appending an ellipsis marker when clipped.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
