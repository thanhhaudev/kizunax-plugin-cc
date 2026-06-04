package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/helper"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/resolver"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols"
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
}

func Run(ctx context.Context, pluginRoot string, p provider.Provider, bundle diff.Bundle, opts Options) (Result, error) {
	schemaJSON, err := schema.LoadSchemaJSON(pluginRoot)
	if err != nil {
		return Result{}, xerrors.Internal("load_schema", "cannot load review schema", err)
	}

	// v0.12: pre-flight enrichment — scan diff symbols, look up definitions
	// in the workspace, attach as referenced files (capped at 256 KiB total).
	// Enrichment is strictly additive: any failure → empty referenced files,
	// main review proceeds.
	if opts.WorkspaceRoot != "" && (len(bundle.Diff) > 0 || len(bundle.Untracked) > 0) {
		symbols.SetWorkspaceRoot(opts.WorkspaceRoot)
		syms := symbols.ExtractFromBundle(bundle)
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "[verbose] scanner: extracted %d symbols\n", len(syms))
		}
		if len(syms) > 0 {
			diffPaths := diff.Paths(bundle)
			stats, rerr := resolver.FindReferences(syms, opts.WorkspaceRoot, diffPaths, 5, 4*1024)
			refs := stats.Refs
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "[warn] resolver: %v\n", rerr)
			} else {
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[verbose] resolver: %d references for %d symbols\n", len(refs), len(syms))
				}
				budget := computePromptBudget(bundle, opts.Glossary, schemaJSON)
				before := len(bundle.Warnings)
				diff.AttachReferenced(&bundle, toReferenceInputs(refs), budget)
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[verbose] bundle: %d referenced files attached (budget=%d bytes)\n", len(bundle.ReferencedFiles), budget)
				}
				for _, w := range bundle.Warnings[before:] {
					if strings.HasPrefix(w, "referenced files dropped") {
						fmt.Fprintf(os.Stderr, "[warn] %s\n", w)
					}
				}
			}
		}
	}

	pr, err := prompt.Build(pluginRoot, opts.Mode, bundle, schemaJSON, opts.Focus, opts.Glossary)
	if err != nil {
		return Result{}, err
	}

	// Pre-flight token guard: estimate input tokens and reject if over budget.
	inputTokens := prompt.EstimateInputTokens(pr.System, pr.User)
	maxInput := config.ModelMaxInputTokens(opts.Model)
	if inputTokens > maxInput {
		return Result{}, xerrors.User(
			"input_too_large",
			fmt.Sprintf("Estimated %d input tokens exceeds model context (%d) for %s.",
				inputTokens, maxInput, opts.Model),
			"Reduce diff scope with --paths, target a smaller --commit, or switch to a model with larger context.")
	}

	req := provider.ChatRequest{
		System:        pr.System,
		Messages:      []provider.Message{{Role: "user", Content: pr.User}},
		Model:         opts.Model,
		Temperature:   opts.Temperature,
		MaxTokens:     opts.MaxTokens,
		JSONSchema:    schemaJSON,
		TryJSONSchema: true,
	}

	resp, err := p.Chat(ctx, req)
	if err != nil {
		return Result{}, err
	}

	review, parseErr := schema.Parse(resp.Content)
	if parseErr != nil {
		req.Messages = append(req.Messages,
			provider.Message{Role: "assistant", Content: resp.Content},
			provider.Message{Role: "user", Content: fmt.Sprintf(
				"Your previous response could not be parsed as JSON.\nError: %v\nReturn ONLY a JSON object matching the schema. No prose, no fences.",
				parseErr,
			)},
		)
		req.TryJSONSchema = false

		resp2, err2 := p.Chat(ctx, req)
		if err2 != nil {
			return Result{}, err2
		}
		review, parseErr = schema.Parse(resp2.Content)
		if parseErr != nil {
			return Result{}, xerrors.Provider("parse_after_retry",
				fmt.Sprintf("could not parse review JSON after retry: %v", parseErr),
				"check raw response in --verbose mode", parseErr)
		}
		resp = resp2
	}

	// Canonicalize finding.File against the diff path set BEFORE the helper
	// call so TL;DR sees the same paths the user will. LLM occasionally
	// emits a basename; we rewrite when unambiguous and warn when not.
	if warns := canonicalizeFindings(review.Findings, diff.Paths(bundle)); len(warns) > 0 {
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "[warn] %s\n", w)
		}
	}

	// Helper TL;DR: separate single-shot call gated by finding count + flags.
	// Any helper failure (including quota=0) → log + tldr="" + continue.
	if shouldSummarize(opts, review.Findings) && opts.HelperCfg.BaseURL != "" && opts.HelperAPIKey != "" {
		if helperQuotaOK(opts.WorkspaceDir, opts.HelperAPIKey) {
			tldr, hErr := helper.Summarize(ctx, opts.HelperCfg, opts.HelperAPIKey, review)
			if hErr != nil {
				fmt.Fprintf(os.Stderr, "[helper] summarize failed: %v\n", hErr)
			} else {
				review.TLDR = tldr
			}
		} else {
			fmt.Fprintf(os.Stderr, "[helper] skipped: Public v1 quota exhausted\n")
		}
	}

	return Result{
		Review:          review,
		InputTokens:     resp.InputTokens,
		OutputTokens:    resp.OutputTokens,
		TotalTokens:     resp.TotalTokens,
		ReferencedFiles: bundle.ReferencedFiles,
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

// toReferenceInputs converts resolver.Reference to diff.ReferenceInput,
// crossing the package boundary without creating an import cycle.
func toReferenceInputs(refs []resolver.Reference) []diff.ReferenceInput {
	out := make([]diff.ReferenceInput, len(refs))
	for i, r := range refs {
		out[i] = diff.ReferenceInput{
			Path:    r.File,
			Excerpt: r.Excerpt,
			Symbols: []string{r.Symbol.Name},
		}
	}
	return out
}

// computePromptBudget returns the remaining bytes available for referenced
// files. v0.12 caps enrichment at 32 KiB total to keep prompts well under
// typical LLM context limits — referenced files are useful but should not
// dominate the prompt or trigger model output truncation.
func computePromptBudget(b diff.Bundle, glossary, schemaJSON string) int {
	const enrichmentCap = 32 * 1024
	return enrichmentCap
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
