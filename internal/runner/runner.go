package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/llmreviewkit/diff"
	"github.com/thanhhaudev/llmreviewkit/engine"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/helper"
	"github.com/thanhhaudev/llmreviewkit/index"
	"github.com/thanhhaudev/llmreviewkit/prompt"
	"github.com/thanhhaudev/llmreviewkit/provider"
	"github.com/thanhhaudev/llmreviewkit/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/llmreviewkit/statedir"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

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
	}

	eng, err := engine.New(engCfg)
	if err != nil {
		return Result{}, xerrors.Internal("engine_new", "engine construction failed", err)
	}

	// Bundle is consumed (engine.Review may mutate it). Capture diff paths
	// BEFORE Review for use in canonicalizeFindings below.
	diffPaths := diff.Paths(bundle)

	rOpts := engine.ReviewOptions{
		Mode:        opts.Mode,
		Focus:       opts.Focus,
		Glossary:    opts.Glossary,
		Model:       opts.Model,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	res, err := eng.Review(ctx, bundle, rOpts)
	if err != nil {
		return Result{}, err
	}

	// KizunaX-specific layering 1: canonicalize finding paths so TL;DR /
	// renderer sees the same paths the user will.
	if warns := canonicalizeFindings(res.Review.Findings, diffPaths); len(warns) > 0 {
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "[warn] %s\n", w)
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
		go spawnBackgroundIndexSync()
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

// spawnBackgroundIndexSync exec's `kizunax index sync` detached. The
// subprocess survives the parent runner.Run exiting. On Unix it gets its
// own process group (Setpgid — see spawn_unix.go) so the parent's SIGINT
// does not propagate; on Windows the default CreateProcess flags already
// detach the child. Output is discarded. Best-effort: any error is
// swallowed because the current review already has its v1 fallback.
func spawnBackgroundIndexSync() {
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
