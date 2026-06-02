package runner

import (
	"context"
	"fmt"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
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
	Model       string
	Temperature float64
	MaxTokens   int
}

func Run(ctx context.Context, pluginRoot string, p provider.Provider, bundle diff.Bundle, opts Options) (Result, error) {
	schemaJSON, err := schema.LoadSchemaJSON(pluginRoot)
	if err != nil {
		return Result{}, xerrors.Internal("load_schema", "cannot load review schema", err)
	}

	pr, err := prompt.Build(pluginRoot, opts.Mode, bundle, schemaJSON, opts.Focus)
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

	return Result{
		Review:       review,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		TotalTokens:  resp.TotalTokens,
	}, nil
}
