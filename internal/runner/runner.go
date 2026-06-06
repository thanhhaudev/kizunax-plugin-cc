package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/helper"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
	"github.com/thanhhaudev/llmreviewkit/diff"
	"github.com/thanhhaudev/llmreviewkit/engine"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/index"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/provider"
	"github.com/thanhhaudev/llmreviewkit/schema"
	"github.com/thanhhaudev/llmreviewkit/statedir"
	"github.com/thanhhaudev/llmreviewkit/symbols"
)

// resolveExtractionPolicy reads KIZUNAX_PHP_EXTRACTOR from the environment
// and returns a corresponding ExtractionPolicy. Unset or unknown values map
// to engine.DefaultExtractionPolicy (Auto strategy + 60s timeout + 64 KiB
// size cap).
//
// Recognized values (case-insensitive):
//   - "" / "auto"      → StrategyAuto    (default)
//   - "phpsyms"        → StrategyPhpsyms (force phpsyms)
//   - "treesitter"     → StrategyTreeSitter (force tree-sitter)
//   - "regex"          → StrategyRegex   (force regex fallback)
func resolveExtractionPolicy() engine.ExtractionPolicy {
	p := engine.DefaultExtractionPolicy()
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KIZUNAX_PHP_EXTRACTOR"))) {
	case "", "auto":
		p.PHP = engine.StrategyAuto
	case "phpsyms":
		p.PHP = engine.StrategyPhpsyms
	case "treesitter":
		p.PHP = engine.StrategyTreeSitter
	case "regex":
		p.PHP = engine.StrategyRegex
	}
	return p
}

type Result struct {
	Review       schema.ReviewResult
	InputTokens  int
	OutputTokens int
	TotalTokens  int

	// ReferencedFiles is the v0.12 enrichment surface — the resolved files
	// attached to the prompt as context. CLI uses this to populate the job
	// record's referenced_file_paths field for observability.
	ReferencedFiles []diff.ReferencedFile
}

type Options struct {
	Mode        prompt.Mode
	Focus       string
	Glossary    string

	// ReviewContext is the loaded body of .kizunax/review-context.md.
	// Empty if no file was found. Passed verbatim to llmreviewkit's
	// ReviewOptions.ReviewContext.
	ReviewContext string

	// ContextPath is the absolute path of the loaded review-context.md.
	// Empty if no file was found. Used for verbose logging.
	ContextPath string

	// ContextModTime is the file mtime. Zero if no file was found.
	// Surfaces in stderr stale warnings.
	ContextModTime time.Time

	// InlineContext is per-review additional context (from --context-text
	// flag, base64-decoded). Concatenated AFTER ReviewContext before
	// passing to the engine.
	InlineContext string

	Model       string
	Temperature float64
	MaxTokens   int

	// Summary controls force the helper TL;DR call. Mutually exclusive at
	// the CLI layer; if both set, NoSummary wins as a safety belt.
	Summary   bool
	NoSummary bool

	// HelperCfg + HelperAPIKey + WorkspaceDir wire the helper call. When
	// HelperCfg.BaseURL is empty or HelperAPIKey is empty, the runner skips
	// the helper call entirely (no error).
	HelperCfg    config.HelperConfig
	HelperAPIKey string
	WorkspaceDir state.WorkspaceDir

	// v0.12+: workspace root for pre-flight enrichment.
	// When empty, enrichment is skipped (review proceeds with diff-only).
	WorkspaceRoot string

	// Verbose toggles stderr stats for pre-flight enrichment (scanner
	// symbol count, resolver matches, attached files / dropped files).
	Verbose bool

	// ForceRescan, when true, deletes the on-disk index before running the
	// index-backed resolver (v0.13). Only meaningful when KIZUNAX_USE_INDEX=1.
	ForceRescan bool

	// BundleLogSink, if non-nil, receives one jsonl line per Review() call.
	// nil disables telemetry. The kizunax CLI wrapper sets this from
	// KIZUNAX_BUNDLE_LOG=1 by opening the rotation-managed log file.
	BundleLogSink io.Writer

	// v0.16.0 — bundle expansion controls. All default false.
	// resolveExpansion (internal/runner/expansion_resolve.go) walks the
	// precedence stack and threads the resolved values into engine.Config.
	ExpandCallers  bool
	ExpandTypeDefs bool
	ExpandTests    bool
	ExpandAll      bool // shortcut: enables all three
	NoExpand       bool // per-call kill switch
}

func Run(ctx context.Context, pluginRoot string, p provider.Provider, bundle diff.Bundle, opts Options) (Result, error) {
	// useIdx is the resolved kizunax-flag (env + state-file precedence).
	// Determined here in the wrapper because the kizunax-specific state
	// directory layout is wrapper concern, not engine concern.
	useIdx := useIndexResolver(opts.WorkspaceDir)

	// For the --rescan path, the wrapper handles synchronous full rebuild
	// BEFORE engine.Review runs so the engine sees a fresh index.
	if opts.ForceRescan && useIdx && opts.WorkspaceDir.Root != "" && opts.WorkspaceRoot != "" {
		idxPath := filepath.Join(opts.WorkspaceDir.Root, "index", "index.json")
		_ = os.Remove(idxPath)
		// Best-effort: ignore errors here. Engine will fall back to v1 if
		// the rebuild fails, and a background sync will retry next time.
		_, _ = index.LoadOrBuild(opts.WorkspaceDir.Root, opts.WorkspaceRoot)
	}

	// Build the engine config. Use StateWorkspaceOverride to inject the
	// already-resolved kizunax workspace dir directly, avoiding any
	// re-hash divergence between internal/state.Resolve and statedir.Resolve.
	var wsOverride *statedir.WorkspaceDir
	if opts.WorkspaceDir.Root != "" {
		inner := statedir.WorkspaceDir{Root: opts.WorkspaceDir.Root}
		wsOverride = &inner
	}

	expandCallers, expandTypeDefs, expandTests := resolveExpansion(opts, opts.WorkspaceDir)
	anyExpand := expandCallers || expandTypeDefs || expandTests

	// v0.22.0: use llmreviewkit v1.2.0's SkipEnrichment + WorkspaceFileCap
	// fields instead of the WorkspaceRoot-blanking hack from v0.19.1. The
	// new fields are explicit, named, documented, and don't disable other
	// workspace-aware features (state dir, index sync, etc.) the way
	// blanking WorkspaceRoot did.
	//
	// SkipEnrichment fires when the user passed --no-expand (or one of
	// the workspace-size auto-paths set NoExpand). WorkspaceFileCap is a
	// belt-and-suspenders: even if NoExpand isn't set, llmreviewkit will
	// auto-degrade enrichment on monorepos > 3000 tracked files and emit
	// its own [warn] line to stderr.
	if opts.NoExpand && opts.Verbose {
		fmt.Fprintln(os.Stderr, "[verbose] enrichment skipped via SkipEnrichment")
	}

	engCfg := engine.Config{
		Provider:               p,
		WorkspaceRoot:          opts.WorkspaceRoot,
		StateWorkspaceOverride: wsOverride,
		PromptRoot:             pluginRoot,
		UseIndex:               useIdx,
		EnrichBudget:           enrichBudgetFor(anyExpand),
		BundleLogSink:          opts.BundleLogSink,
		Verbose:                opts.Verbose,
		ExpandCallers:          expandCallers,
		ExpandTypeDefs:         expandTypeDefs,
		ExpandTests:            expandTests,
		SkipEnrichment:         opts.NoExpand,
		WorkspaceFileCap:       3000,
		ExtractionPolicy:       resolveExtractionPolicy(),
	}

	eng, err := engine.New(engCfg)
	if err != nil {
		return Result{}, xerrors.Internal("engine_new", "engine construction failed", err)
	}

	// Wire a per-review extract observer for verbose telemetry. The observer is
	// process-singleton (one slot in symbols package) — kizunax owns the slot
	// during a Review and tears it down on exit so multiple invocations don't
	// leak state.
	extractCounts := map[string]int{}
	extractTotalNanos := map[string]int64{}
	symbols.SetExtractObserver(func(ev symbols.ExtractEvent) {
		name := symbols.ExtractStrategyName(ev.Strategy)
		extractCounts[name]++
		extractTotalNanos[name] += ev.Duration.Nanoseconds()
	})
	defer symbols.SetExtractObserver(nil)

	// Bundle is consumed (engine.Review may mutate it). Capture diff paths
	// BEFORE Review for use in canonicalizeFindings below.
	diffPaths := diff.Paths(bundle)

	rOpts := engine.ReviewOptions{
		Mode:           opts.Mode,
		Focus:          opts.Focus,
		Glossary:       opts.Glossary,
		ReviewContext:  combinedContext(opts.ReviewContext, opts.InlineContext),
		ContextPath:    opts.ContextPath,
		ContextModTime: opts.ContextModTime,
		Model:          opts.Model,
		Temperature:    opts.Temperature,
		MaxTokens:      opts.MaxTokens,
	}
	res, err := eng.Review(ctx, bundle, rOpts)
	if err != nil {
		return Result{}, err
	}

	if opts.Verbose {
		// Deterministic order so output is testable.
		order := []string{"phpsyms", "treesitter", "regex", "auto", "unknown"}
		for _, name := range order {
			count := extractCounts[name]
			if count == 0 {
				continue
			}
			avgMs := float64(extractTotalNanos[name]) / float64(count) / 1e6
			fmt.Fprintf(os.Stderr, "[verbose] PHP extractor: %s, %d files, avg %.1fms/file\n", name, count, avgMs)
		}
	}

	// KizunaX-specific layering 1: canonicalize finding paths so TL;DR /
	// renderer sees the same paths the user will.
	if warns := canonicalizeFindings(res.Review.Findings, diffPaths); len(warns) > 0 {
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "[warn] %s\n", w)
		}
	}

	// v0.24.0: post-process finding verification via llmreviewkit v1.4.0.
	// Classifies each finding as in-hunk / context-only / file-not-in-diff
	// and emits a stderr summary so the user sees signal:noise at a glance.
	// Does NOT modify the findings — purely diagnostic. Callers who want
	// stricter behavior can grep this output or post-filter themselves.
	if len(res.Review.Findings) > 0 {
		verifications := engine.VerifyFindings(bundle, res.Review.Findings)
		inHunk, contextOnly, notInDiff := 0, 0, 0
		for _, v := range verifications {
			switch {
			case v.InHunk:
				inHunk++
			case v.FileInDiff:
				contextOnly++
			default:
				notInDiff++
			}
		}
		total := len(verifications)
		if contextOnly > 0 || notInDiff > 0 {
			fmt.Fprintf(os.Stderr, "[info] finding verification: %d/%d in changed hunks, %d in unchanged context (LLM line drift), %d cite files not in diff (likely hallucinated)\n",
				inHunk, total, contextOnly, notInDiff)
			if notInDiff > 0 {
				for i, v := range verifications {
					if !v.FileInDiff {
						f := res.Review.Findings[i]
						fmt.Fprintf(os.Stderr, "[warn]   hallucinated finding %d: %q at %s:%d-%d — file not in diff\n",
							i+1, f.Title, f.File, f.LineStart, f.LineEnd)
					}
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "[info] finding verification: %d/%d in changed hunks (all verified)\n", inHunk, total)
		}
	}

	// KizunaX-specific layering 2: helper TL;DR (gated by finding count + flags + quota).
	if shouldSummarize(opts, res.Review.Findings) && opts.HelperCfg.BaseURL != "" && opts.HelperAPIKey != "" {
		if helperQuotaOK(opts.WorkspaceDir, opts.HelperAPIKey) {
			tldr, hErr := helper.Summarize(ctx, opts.HelperCfg, opts.HelperAPIKey, res.Review)
			if hErr != nil {
				fmt.Fprintf(os.Stderr, "[helper] summarize failed: %v\n", hErr)
			} else {
				res.Review.TLDR = tldr
			}
		} else {
			fmt.Fprintf(os.Stderr, "[helper] skipped: Public v1 quota exhausted\n")
		}
	}

	// KizunaX-specific layering 3: if v2 was requested but engine fell
	// back to v1 (index missing/stale), spawn a detached background sync
	// so the NEXT review uses v2. Matches v0.13.0 lazy-background design.
	if useIdx && res.Stats.ResolverPath == "v1" && opts.WorkspaceDir.Root != "" && opts.WorkspaceRoot != "" {
		go spawnBackgroundIndexSync(opts.WorkspaceDir)
	}

	return Result{
		Review:          res.Review,
		InputTokens:     res.InputTokens,
		OutputTokens:    res.OutputTokens,
		TotalTokens:     res.TotalTokens,
		ReferencedFiles: res.ReferencedFiles,
	}, nil
}

func shouldSummarize(opts Options, findings []schema.Finding) bool {
	if opts.NoSummary {
		return false
	}
	if opts.Summary {
		return true
	}
	return len(findings) >= 3
}

// helperQuotaOK returns true unless we have cached evidence that the helper
// key's Public v1 (credits) quota is exhausted. Cache miss → fail-open (true).
func helperQuotaOK(ws state.WorkspaceDir, apiKey string) bool {
	if ws.Root == "" || apiKey == "" {
		return true
	}
	entry, ok := usage.LoadCachedEntry(ws, apiKey)
	if !ok {
		return true
	}
	if entry.Credits != nil && !entry.Credits.Unlimited && entry.Credits.Remaining <= 0 {
		return false
	}
	return true
}

// enrichBudgetFor returns the EnrichBudget passed to engine.Config.
// When any expansion strategy is enabled, return 0 so engine.New
// auto-bumps to 96 KB (the v1.1.0 default for expansion). Otherwise
// preserve the v0.15.0 32 KB baseline.
func enrichBudgetFor(anyExpand bool) int {
	if anyExpand {
		return 0
	}
	return 32 * 1024
}

// combinedContext concatenates the file-based review-context with the
// per-review inline context. Inline context is appended below the file
// content with a separator + heading. Either may be empty.
func combinedContext(file, inline string) string {
	if file == "" && inline == "" {
		return ""
	}
	if inline == "" {
		return file
	}
	if file == "" {
		return "## Per-review notes\n\n" + inline
	}
	return file + "\n\n---\n\n## Per-review notes\n\n" + inline
}

// useIndexResolver returns true if v0.13 index-backed resolver should be
// tried. Precedence: KIZUNAX_DISABLE_INDEX kill switch > KIZUNAX_USE_INDEX
// env > per-workspace state file > default false.
// v0.13.0 default is opt-in; v0.13.2 will flip to opt-out.
func useIndexResolver(ws state.WorkspaceDir) bool {
	if os.Getenv("KIZUNAX_DISABLE_INDEX") == "1" {
		return false
	}
	if os.Getenv("KIZUNAX_USE_INDEX") == "1" {
		return true
	}
	// Fallback to persisted per-workspace flag.
	if ws.Root != "" {
		if s, err := state.LoadUseIndex(ws); err == nil && s.Enabled {
			return true
		}
	}
	return false
}

// indexSyncStaleAfter is how long an in-flight index-sync marker is
// considered fresh before the next caller assumes it crashed and tries
// again. 30 minutes is well above any reasonable sync time on any
// workspace we've measured (largest seen: ~3 min on a 5000-file repo).
const indexSyncStaleAfter = 30 * time.Minute

// spawnBackgroundIndexSync exec's `kizunax index sync` detached. The
// subprocess survives the parent runner.Run exiting. On Unix it gets its
// own process group (Setpgid — see spawn_unix.go) so the parent's SIGINT
// does not propagate; on Windows the default CreateProcess flags already
// detach the child. Output is discarded. Best-effort: any error is
// swallowed because the current review already has its v1 fallback.
//
// v0.19.0: cross-process dedup via state/{ws}/index.sync.inflight marker.
// If a sync was started < indexSyncStaleAfter ago, skip — another process
// is handling it. Without this, fan-out spawns 9 concurrent `index sync`
// children that all hammer the filesystem and CPU simultaneously (the
// root cause of the M1 Pro thermal trip seen in v0.18.0 testing).
func spawnBackgroundIndexSync(ws state.WorkspaceDir) {
	if ws.Root != "" {
		markerPath := filepath.Join(ws.Root, "index.sync.inflight")
		if info, err := os.Stat(markerPath); err == nil && time.Since(info.ModTime()) < indexSyncStaleAfter {
			return // Another sync is in flight (or just finished). Skip.
		}
		// Touch the marker. The sibling sync subprocess does the actual
		// work; we only signal "a sync was kicked off at this time".
		// We don't remove on completion — the modtime + stale window
		// is enough, and avoids needing the child to know about cleanup.
		if f, err := os.OpenFile(markerPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
			f.Close()
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "index", "sync")
	cmd.SysProcAttr = detachSysProcAttr()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	_ = cmd.Start() // intentional: parent must not wait on this
}
