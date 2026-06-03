package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/helper"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

type Result struct {
	Review       schema.ReviewResult
	InputTokens  int
	OutputTokens int
	TotalTokens  int
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
}

func Run(ctx context.Context, pluginRoot string, p provider.Provider, bundle diff.Bundle, opts Options) (Result, error) {
	schemaJSON, err := schema.LoadSchemaJSON(pluginRoot)
	if err != nil {
		return Result{}, xerrors.Internal("load_schema", "cannot load review schema", err)
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
		Review:       review,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		TotalTokens:  resp.TotalTokens,
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
