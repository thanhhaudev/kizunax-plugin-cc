package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/llmreviewkit/diff"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/git"
	"github.com/thanhhaudev/llmreviewkit/glossary"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
	"github.com/thanhhaudev/llmreviewkit/bundlelog"
)

// kindFromMode maps a prompt mode to the corresponding job.Kind.
func kindFromMode(mode prompt.Mode) job.Kind {
	if mode == prompt.ModeAdversarial {
		return job.KindAdversarialReview
	}
	return job.KindReview
}

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
	if hasFlag(args, "--background") {
		fmt.Fprintln(os.Stderr, "[kizunax] --background is deprecated since v0.9 (no-op); async is delegated to Claude Code's Bash(run_in_background:true)")
	}
	focus := flagValue(args, "--focus")
	providerOverride := flagValue(args, "--provider")
	summary := hasFlag(args, "--summary")
	noSummary := hasFlag(args, "--no-summary")
	rescan := hasFlag(args, "--rescan")
	expandCallers := hasFlag(args, "--expand-callers")
	expandTypeDefs := hasFlag(args, "--expand-typedefs")
	expandTests := hasFlag(args, "--expand-tests")
	expandAll := hasFlag(args, "--expand-all")
	noExpand := hasFlag(args, "--no-expand")
	if summary && noSummary {
		return xerrors.User("conflict_summary_flags",
			"--summary and --no-summary are mutually exclusive", "")
	}

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

	if target.Kind == git.TargetBranchDiff {
		resolved, substituted, err := resolveBaseRef(target.Base)
		if err != nil {
			return xerrors.User("base_ref_not_found", err.Error(), "Run `git branch -a` to see available refs.")
		}
		if substituted {
			fmt.Fprintf(os.Stderr, "[info] base ref %q not found locally; using %q (repo default branch) instead\n", target.Base, resolved)
			target.Base = resolved
		}
	}

	// v0.19.0 heat-aware enrichment guard: on monorepos with thousands of
	// tracked files, the v0.12 workspace symbol enrichment (WASM tree-sitter
	// parse + walker) can spin the binary at 100% CPU for 10+ minutes before
	// the LLM call. Auto-disable expansion unless the user explicitly opted
	// in with --expand-all or any --expand-*.
	if !noExpand && !expandAll && !expandCallers && !expandTypeDefs && !expandTests {
		if count, ok := countTrackedFilesCheaply(cwd); ok && count > enrichmentWorkspaceCap {
			fmt.Fprintf(os.Stderr, "[warn] Workspace has %d tracked files (cap %d); auto-skipping symbol enrichment to avoid CPU spin.\n", count, enrichmentWorkspaceCap)
			fmt.Fprintln(os.Stderr, "[warn] Override with --expand-all (or any --expand-*) if you accept the slowdown.")
			noExpand = true
		}
	}

	cfg, err := config.Load(providerOverride)
	if err != nil {
		return err
	}

	gloss, glossErr := glossary.Load(cwd)
	if glossErr != nil {
		fmt.Fprintf(os.Stderr, "[warn] glossary: %v\n", glossErr)
	}
	if verbose {
		if gloss.Path == "" {
			fmt.Fprintln(os.Stderr, "[verbose] glossary: no glossary file found in workspace")
		} else {
			suffix := ""
			if gloss.Truncated {
				suffix = " (truncated)"
			}
			fmt.Fprintf(os.Stderr, "[verbose] glossary: loaded %d chars from %s%s\n",
				len(gloss.Content), gloss.Path, suffix)
		}
	}
	if gloss.Truncated {
		fmt.Fprintf(os.Stderr, "[warn] glossary truncated to %d bytes: %s\n", len(gloss.Content), gloss.Path)
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

	// Surface bundle warnings prominently BEFORE the LLM call so the user
	// can Ctrl+C and re-run with --paths if they see truncation. Without
	// this the warnings only appear inside the rendered review, too late
	// for a multi-minute API call on a giant truncated diff.
	if len(bundle.Warnings) > 0 {
		fmt.Fprintln(os.Stderr, "[warn] Bundle warnings — review may miss context:")
		for _, w := range bundle.Warnings {
			fmt.Fprintf(os.Stderr, "[warn]   - %s\n", w)
		}
		fmt.Fprintln(os.Stderr, "[warn] Tip: re-run with --paths <dir1,dir2,...> to narrow scope (e.g. --paths app/Http,app/Services).")
	}

	pluginRoot, err := ResolvePluginRoot()
	if err != nil {
		return err
	}

	p, err := buildProvider(cfg)
	if err != nil {
		return err
	}

	var wsDir state.WorkspaceDir
	if ws, wsErr := state.Resolve(cwd); wsErr == nil {
		wsDir = ws
	}

	var bundleSink io.Writer
	if os.Getenv("KIZUNAX_BUNDLE_LOG") == "1" && wsDir.Root != "" {
		logPath := filepath.Join(wsDir.Root, bundlelog.LogName)
		backupPath := filepath.Join(wsDir.Root, bundlelog.BackupName)
		// Rotate before opening for append — preserves v0.12.4 10 MiB cap behavior.
		if info, err := os.Stat(logPath); err == nil && info.Size() >= bundlelog.SizeCapBytes {
			_ = os.Rename(logPath, backupPath)
		}
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			bundleSink = f
			defer f.Close()
		} else {
			fmt.Fprintf(os.Stderr, "[warn] could not open bundle log %s: %v\n", logPath, err)
		}
	}

	ctx := context.Background()
	start := time.Now()
	result, runErr := runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:          mode,
		Focus:         focus,
		Glossary:      gloss.Content,
		Model:         cfg.Model,
		Temperature:   cfg.Temperature,
		MaxTokens:     cfg.MaxTokens,
		Summary:       summary,
		NoSummary:     noSummary,
		HelperCfg:     cfg.Helper,
		HelperAPIKey:  cfg.HelperAPIKey,
		WorkspaceDir:  wsDir,
		WorkspaceRoot: cwd,
		Verbose:       verbose,
		ForceRescan:   rescan,
		ExpandCallers: expandCallers,
		ExpandTypeDefs: expandTypeDefs,
		ExpandTests:   expandTests,
		ExpandAll:     expandAll,
		NoExpand:      noExpand,
		BundleLogSink: bundleSink,
	})
	end := time.Now()
	dur := end.Sub(start)

	if verbose {
		if runErr == nil {
			fmt.Fprintf(os.Stderr, "[verbose] tokens in=%d out=%d total=%d\n",
				result.InputTokens, result.OutputTokens, result.TotalTokens)
		}
		fmt.Fprintf(os.Stderr, "[verbose] duration=%dms model=%s\n",
			dur.Milliseconds(), cfg.Model)
	}

	// Persist a job record so /kizunax:status, /result, /cancel work for this
	// review. Foreground reviews are still 1-shot, but the record gives audit
	// + retrieval. Best-effort: a save failure must not fail the review.
	record := job.Job{
		ID:          job.NewID(),
		Kind:        kindFromMode(mode),
		SessionID:   CurrentSessionID(),
		CreatedAt:   start,
		StartedAt:   start,
		CompletedAt: &end,
		DurationMs:  dur.Milliseconds(),
		Request: job.Request{
			Mode:          mode.String(),
			Target:        target,
			Focus:         focus,
			Provider:      cfg.Provider,
			Model:         cfg.Model,
			Summary:       summary,
			NoSummary:     noSummary,
			HelperBaseURL: cfg.Helper.BaseURL,
			HelperModel:   cfg.Helper.Model,
			HelperKeyHash: helperKeyHash(cfg.HelperAPIKey),
			HelperKeyMask: helperKeyMask(cfg.HelperAPIKey),
			// v0.12: paths only (privacy).
			ReferencedFilePaths: referencedFilePathsFromResult(result),
		},
		LogPath:  "",
		Warnings: bundle.Warnings,
	}
	if runErr != nil {
		record.Status = job.StatusFailed
		record.Error = runErr.Error()
	} else {
		record.Status = job.StatusCompleted
		review := result.Review
		record.Result = &review
		if result.TotalTokens > 0 || result.InputTokens > 0 || result.OutputTokens > 0 {
			record.Tokens = &job.TokenUsage{
				Input:  result.InputTokens,
				Output: result.OutputTokens,
				Total:  result.TotalTokens,
			}
		}
	}
	if ws, wsErr := state.Resolve(cwd); wsErr == nil {
		if err := job.Save(ws, record); err != nil {
			fmt.Fprintf(os.Stderr, "[kizunax] warning: could not persist job record: %v\n", err)
		}
	}

	if runErr != nil {
		return runErr
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

// helperKeyHash returns the sha256 hex of the helper key, or "" when no
// helper key was resolved (so omitempty drops the field from the job JSON).
func helperKeyHash(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	return usage.HashKey(apiKey)
}

// helperKeyMask returns the display-safe mask of the helper key, or ""
// when no helper key was resolved.
func helperKeyMask(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	return usage.MaskKey(apiKey)
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

// enrichmentWorkspaceCap is the per-workspace tracked-file count above
// which symbol enrichment is auto-skipped. Empirically tuned against
// a Laravel monorepo (Oneplat B2B System, ~5000 PHP files) where the
// v0.12 WASM walker spun the binary at 100% CPU for 11+ minutes.
const enrichmentWorkspaceCap = 3000

// countTrackedFilesCheaply uses `git ls-files -z` to count tracked files
// in the repo at cwd. Runs in <1s on a 10k-file monorepo (vs the symbol
// walker's 10+ minutes). Returns (count, true) on success or (0, false)
// if git fails for any reason (caller should NOT block on this).
func countTrackedFilesCheaply(cwd string) (int, bool) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	return bytes.Count(out, []byte{0}), true
}

// resolveBaseRef verifies that ref exists locally. If not, it falls back to
// the repo's default branch from origin/HEAD. Returns the resolved ref, a
// flag indicating whether a substitution happened, and an error if no usable
// ref could be found.
func resolveBaseRef(ref string) (string, bool, error) {
	if ref == "" {
		return "", false, nil
	}
	if exec.Command("git", "rev-parse", "--verify", ref+"^{commit}").Run() == nil {
		return ref, false, nil
	}
	out, err := exec.Command("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output()
	if err != nil {
		return "", false, fmt.Errorf("base ref %q not found locally and no remote default branch detected", ref)
	}
	fallback := strings.TrimSpace(string(out))
	fallback = strings.TrimPrefix(fallback, "origin/")
	if fallback == "" {
		return "", false, fmt.Errorf("base ref %q not found locally; remote default branch is empty", ref)
	}
	if exec.Command("git", "rev-parse", "--verify", fallback+"^{commit}").Run() != nil {
		return "", false, fmt.Errorf("base ref %q not found locally; default branch %q also not found", ref, fallback)
	}
	return fallback, true, nil
}

func referencedFilePathsFromResult(r runner.Result) []string {
	out := make([]string, len(r.ReferencedFiles))
	for i, f := range r.ReferencedFiles {
		out[i] = f.Path
	}
	return out
}
