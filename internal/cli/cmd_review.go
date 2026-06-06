package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
	"github.com/thanhhaudev/llmreviewkit/bundlelog"
	llmcontext "github.com/thanhhaudev/llmreviewkit/context"
	"github.com/thanhhaudev/llmreviewkit/diff"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/git"
	"github.com/thanhhaudev/llmreviewkit/glossary"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/render"
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

	// v0.26.0: new flags for fan-out dispatch.
	strategy := flagValue(args, "--strategy")
	if strategy == "" {
		strategy = "auto"
	}
	switch strategy {
	case "auto", "single", "fanout":
		// ok
	default:
		return xerrors.User("invalid_strategy",
			fmt.Sprintf("unknown --strategy=%q; want auto|single|fanout", strategy), "")
	}
	modelOverride := flagValue(args, "--model")
	jsonOutput := hasFlag(args, "--json")

	contextTextRaw := flagValue(args, "--context-text")
	var inlineContext string
	if contextTextRaw != "" {
		if decoded, derr := base64.StdEncoding.DecodeString(contextTextRaw); derr == nil {
			inlineContext = string(decoded)
		} else {
			// Graceful fallback: treat as plain text.
			inlineContext = contextTextRaw
		}
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

	// v0.21.0 smart default: if the user passed NO target flag (parseTarget
	// fell through to TargetWorkingTree by default) AND the working tree is
	// clean, flip to a branch-diff review against the auto-detected base.
	// This avoids the "nothing to review" dead end on a clean checkout —
	// users typing `/kizunax:review` from a feature branch almost always
	// want their PR commits reviewed, not nothing.
	if target.Kind == git.TargetWorkingTree && !hasFlag(args, "--working-tree") {
		if dirty, ok := isWorkingTreeDirty(cwd); ok && !dirty {
			fmt.Fprintln(os.Stderr, "[info] no target flag and working tree is clean — defaulting to branch diff vs --base auto")
			target = git.Target{Kind: git.TargetBranchDiff, Base: "auto", Paths: target.Paths}
		}
	}

	if target.Kind == git.TargetBranchDiff {
		// v0.20.0: --base auto triggers smart detection — upstream tracking
		// branch first, then common dev branch names, finally origin/HEAD.
		// Helps users on PR workflows (feature → develop → master) where
		// `--base master` includes hundreds of unrelated commits from the
		// integration branch.
		if target.Base == "auto" {
			resolved, err := autoDetectBaseRef()
			if err != nil {
				return xerrors.User("base_auto_failed", err.Error(),
					"Pass --base <ref> explicitly, or set an upstream with `git branch --set-upstream-to=<remote>/<branch>`.")
			}
			fmt.Fprintf(os.Stderr, "[info] --base auto resolved to %q\n", resolved)
			target.Base = resolved
		}
		resolved, substituted, err := resolveBaseRef(target.Base)
		if err != nil {
			return xerrors.User("base_ref_not_found", err.Error(), "Run `git branch -a` to see available refs.")
		}
		if substituted {
			fmt.Fprintf(os.Stderr, "[info] base ref %q not found locally; using %q (repo default branch) instead\n", target.Base, resolved)
			target.Base = resolved
		}
	}

	// v0.22.0: the kizunax-side workspace size guard from v0.19.0 was
	// moved upstream into llmreviewkit v1.2.0 (engine.Config.WorkspaceFileCap).
	// runner.go always sets WorkspaceFileCap = 3000, so the engine emits
	// its own auto-degrade warning when the cap is exceeded.
	// We still locally set noExpand if the user explicitly asked, which
	// flows into engine.Config.SkipEnrichment via runner.

	cfg, err := config.Load(providerOverride)
	if err != nil {
		return err
	}
	if modelOverride != "" {
		cfg.Model = modelOverride
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] --model override: %s\n", modelOverride)
		}
	}

	gloss, glossErr := glossary.Load(cwd, []string{
		filepath.Join(".kizunax", "glossary.md"),
		filepath.Join("docs", "glossary.md"),
		"GLOSSARY.md",
	})
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

	revCtx, ctxErr := llmcontext.Load(cwd, []string{
		filepath.Join(".kizunax", "review-context.md"),
		filepath.Join("docs", "review-context.md"),
		"REVIEW-CONTEXT.md",
	})
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "[warn] review-context: %v\n", ctxErr)
	}
	if revCtx.Path == "" {
		fmt.Fprintln(os.Stderr, "[info] No review-context.md found. Run /kizunax:context to generate one.")
	} else if time.Since(revCtx.ModTime) > 14*24*time.Hour {
		days := int(time.Since(revCtx.ModTime).Hours() / 24)
		fmt.Fprintf(os.Stderr, "[warn] review-context.md is %d days old. Run /kizunax:context to refresh.\n", days)
	}
	if verbose {
		if revCtx.Path == "" {
			fmt.Fprintln(os.Stderr, "[verbose] review-context: no file in workspace")
		} else {
			suffix := ""
			if revCtx.Truncated {
				suffix = " (truncated)"
			}
			fmt.Fprintf(os.Stderr, "[verbose] review-context: loaded %d chars from %s%s\n",
				len(revCtx.Content), revCtx.Path, suffix)
		}
		if inlineContext != "" {
			fmt.Fprintf(os.Stderr, "[verbose] review-context: %d chars inline via --context-text\n",
				len(inlineContext))
		}
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

	// v0.26.0: resolve ctx + wsDir early so the fanout path can use them.
	ctx := context.Background()
	var wsDir state.WorkspaceDir
	if ws, wsErr := state.Resolve(cwd); wsErr == nil {
		wsDir = ws
	}

	// v0.26.0: decide fan-out based on --strategy + diff size.
	// Workers (--json) never recursively fan-out — prevents infinite spawning.
	shouldFanout := false
	if !jsonOutput {
		switch strategy {
		case "fanout":
			shouldFanout = true
		case "auto":
			// Fan-out when diff is large enough to warrant parallel review.
			// Threshold: >150 KB OR >100 files.
			fileCount := countChangedFiles(cwd, target)
			if bundle.TotalBytes > 150*1024 || fileCount > 100 {
				shouldFanout = true
			}
		}
	}

	if shouldFanout {
		return runFanoutReview(ctx, runFanoutArgs{
			cwd:          cwd,
			target:       target,
			bundle:       bundle,
			cfg:          cfg,
			mode:         mode,
			focus:        focus,
			verbose:      verbose,
			quiet:        quiet,
			originalArgs: args,
			wsDir:        wsDir,
		})
	}

	// v0.20.0 baseline-mismatch warning: if the user passed an explicit
	// --base and the resulting diff is large, check whether an alternate
	// base (upstream, develop, dev, main) would produce a meaningfully
	// smaller diff — a strong signal the user intended a PR-scope review
	// against the integration branch, not against master/main.
	if target.Kind == git.TargetBranchDiff {
		out, err := exec.Command("git", "diff", "--name-only", target.Base+"...HEAD").Output()
		if err == nil {
			fileCount := strings.Count(string(out), "\n")
			if suggested, suggestedCount, ok := suggestSmallerBaseRefs(target.Base, fileCount); ok {
				fmt.Fprintf(os.Stderr, "[warn] Diff vs %q is %d files; vs %q would be %d files.\n", target.Base, fileCount, suggested, suggestedCount)
				fmt.Fprintf(os.Stderr, "[warn] If this PR merges into %q, re-run with --base %s for a focused review.\n", suggested, suggested)
			}
		}
	}

	pluginRoot, err := ResolvePluginRoot()
	if err != nil {
		return err
	}

	p, err := buildProvider(cfg)
	if err != nil {
		return err
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

	start := time.Now()
	result, runErr := runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:           mode,
		Focus:          focus,
		Glossary:       gloss.Content,
		ReviewContext:  revCtx.Content,
		ContextPath:    revCtx.Path,
		ContextModTime: revCtx.ModTime,
		InlineContext:  inlineContext,
		Model:          cfg.Model,
		Temperature:    cfg.Temperature,
		MaxTokens:      cfg.MaxTokens,
		Summary:        summary,
		NoSummary:      noSummary,
		HelperCfg:      cfg.Helper,
		HelperAPIKey:   cfg.HelperAPIKey,
		WorkspaceDir:   wsDir,
		WorkspaceRoot:  cwd,
		Verbose:        verbose,
		ForceRescan:    rescan,
		ExpandCallers:  expandCallers,
		ExpandTypeDefs: expandTypeDefs,
		ExpandTests:    expandTests,
		ExpandAll:      expandAll,
		NoExpand:       noExpand,
		BundleLogSink:  bundleSink,
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

	// v0.26.0: --json mode — worker subprocess path. Emit ReviewResult as JSON
	// to stdout so the parent fan-out process can decode it. Skip usage refresh
	// and footer; the parent handles those once after merging all workers.
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(result.Review); err != nil {
			return xerrors.Internal("json_encode", "could not encode review result", err)
		}
		return nil
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

// isWorkingTreeDirty reports whether the repo at cwd has any uncommitted
// changes (modified, staged, or untracked) per `git status --porcelain`.
// Returns (dirty, true) on success or (false, false) on any git error.
// On failure the v0.21.0 caller treats it as "don't change behavior" so a
// broken/non-git workspace doesn't accidentally flip to --base auto.
func isWorkingTreeDirty(cwd string) (bool, bool) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return len(strings.TrimSpace(string(out))) > 0, true
}

// autoDetectBaseRef chooses the best base ref for a branch-diff review.
// Precedence (first one that resolves wins):
//  1. The current branch's upstream tracking branch (`@{upstream}`),
//     UNLESS it is just `<remote>/<current-branch>` — that's the same
//     branch's remote copy, not a useful PR base. v0.22.1 fix after a
//     real-world report: on feature/compare_order_phase2 the binary
//     picked origin/feature/compare_order_phase2 and produced a 0-diff
//     "review" of a single local spec file.
//  2. Common dev branch names: develop, dev, main, master
//  3. The repo's remote default branch from origin/HEAD
//
// Returns an error only if NONE of these resolve.
func autoDetectBaseRef() (string, error) {
	currentBranch := ""
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		currentBranch = strings.TrimSpace(string(out))
	}

	// 1. Upstream tracking branch, unless it's just the same branch's remote.
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		if ref != "" && !isSelfUpstream(ref, currentBranch) {
			if exec.Command("git", "rev-parse", "--verify", ref+"^{commit}").Run() == nil {
				return ref, nil
			}
		}
	}
	// 2. Common dev branch names — skip if it's the current branch itself.
	for _, candidate := range []string{"develop", "dev", "main", "master"} {
		if candidate == currentBranch {
			continue
		}
		if exec.Command("git", "rev-parse", "--verify", candidate+"^{commit}").Run() == nil {
			return candidate, nil
		}
	}
	// 3. Remote default branch from origin/HEAD.
	if out, err := exec.Command("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		if ref != "" && !isSelfUpstream(ref, currentBranch) {
			if exec.Command("git", "rev-parse", "--verify", ref+"^{commit}").Run() == nil {
				return ref, nil
			}
		}
	}
	return "", fmt.Errorf("could not auto-detect a base ref: no upstream tracking (or upstream is self), no develop/dev/main/master branch, no origin/HEAD")
}

// isSelfUpstream returns true when upstream is just `<remote>/<currentBranch>`
// — i.e., the remote copy of the same branch, which is not a useful base for
// "what does this PR change". Examples:
//   - upstream="origin/feature/x" currentBranch="feature/x" -> true (self)
//   - upstream="origin/develop"   currentBranch="feature/x" -> false (real base)
//   - upstream="origin/master"    currentBranch="master"    -> true (self, on default branch)
func isSelfUpstream(upstream, currentBranch string) bool {
	if currentBranch == "" {
		return false
	}
	if upstream == currentBranch {
		return true
	}
	// upstream typically looks like "<remote>/<branch>". Strip the first
	// segment and compare. Handles slashes inside branch names (e.g.
	// "feature/x") because we only trim ONE remote prefix.
	if idx := strings.Index(upstream, "/"); idx > 0 {
		if upstream[idx+1:] == currentBranch {
			return true
		}
	}
	return false
}

// suggestSmallerBaseRefs returns alternate base refs that produce a
// significantly smaller diff than the chosen base. Used to warn the user
// when `--base master` includes hundreds of files from an integration
// branch and `--base develop` would scope to the PR's actual commits.
//
// Returns (suggestion, suggestionFileCount, ok). ok is false when no
// alternate produces a meaningfully smaller diff (less than 1/3 the
// chosen base's file count, and the absolute difference is > 20 files).
func suggestSmallerBaseRefs(currentBase string, currentFileCount int) (string, int, bool) {
	if currentFileCount < 100 {
		// Small diff already, nothing to suggest.
		return "", 0, false
	}

	currentBranch := ""
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		currentBranch = strings.TrimSpace(string(out))
	}

	candidates := []string{"@{upstream}", "develop", "dev", "main", "master"}
	bestRef := ""
	bestCount := currentFileCount
	for _, c := range candidates {
		if c == currentBase {
			continue
		}
		// Resolve abstract refs (e.g. @{upstream}) to a concrete name first
		// so the same ref doesn't compete with itself.
		resolved := c
		if c == "@{upstream}" {
			out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}").Output()
			if err != nil {
				continue
			}
			resolved = strings.TrimSpace(string(out))
			if resolved == "" || resolved == currentBase {
				continue
			}
		}
		// v0.22.1: skip self-upstream candidates. origin/feature/x while
		// on feature/x produces a near-empty diff and is never a useful
		// PR base; suggesting it sends users in circles.
		if isSelfUpstream(resolved, currentBranch) {
			continue
		}
		// Verify it exists.
		if exec.Command("git", "rev-parse", "--verify", resolved+"^{commit}").Run() != nil {
			continue
		}
		// Count files in diff vs this candidate.
		out, err := exec.Command("git", "diff", "--name-only", resolved+"...HEAD").Output()
		if err != nil {
			continue
		}
		count := strings.Count(string(out), "\n")
		if count > 0 && count < bestCount {
			bestRef = resolved
			bestCount = count
		}
	}
	// Only suggest if alternate is < 1/3 current AND absolute saving > 20 files.
	if bestRef != "" && bestCount*3 < currentFileCount && currentFileCount-bestCount > 20 {
		return bestRef, bestCount, true
	}
	return "", 0, false
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

// countChangedFiles returns the number of changed files for target using git.
// Used only for the auto-strategy threshold; diff.Collect is authoritative for
// content. Returns 0 on error (caller falls back to byte-size threshold only).
func countChangedFiles(cwd string, target git.Target) int {
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
		return 0
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}

// filterStrategyFlag removes --strategy and its value from args.
// Used when falling back to single-review from within fan-out logic.
func filterStrategyFlag(args []string) []string {
	var out []string
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--strategy" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "--strategy=") {
			continue
		}
		out = append(out, a)
	}
	return out
}
