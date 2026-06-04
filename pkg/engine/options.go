// Package engine provides the public review pipeline entrypoint for
// llmreviewkit. Construct an Engine via New(cfg) and call Review() per
// diff to be reviewed. Sub-package APIs (diff, prompt, schema, render,
// etc.) remain available for callers who want to assemble a custom
// pipeline.
package engine

import (
	"io"

	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/provider"
)

// Config configures an Engine. Provider and WorkspaceRoot are required;
// all other fields have sensible defaults.
type Config struct {
	// Required.

	// Provider is the LLM backend. Any implementation of provider.Provider
	// works — bundled openai/anthropic adapters or a user-written impl
	// for Bedrock, Vertex, Ollama, etc.
	Provider provider.Provider

	// WorkspaceRoot is the absolute path to the project being reviewed.
	WorkspaceRoot string

	// Optional.

	// StateDir is the on-disk directory base for index files + telemetry.
	// If empty, defaults to os.TempDir() + "/llmreviewkit". The actual
	// workspace state lives under <StateDir>/<workspace-hash>/.
	StateDir string

	// PromptRoot is a directory containing custom prompt templates. If
	// empty, embedded defaults are used (pkg/prompt/embedded/*.md).
	PromptRoot string

	// UseIndex enables the v0.13 index-backed resolver. Default false
	// (regex resolver used). When true and no usable index exists,
	// Review() falls back to v1 transparently for the current call.
	UseIndex bool

	// EnrichBudget caps the bytes of referenced-file content attached to
	// the prompt. Default 32*1024 (32 KB).
	EnrichBudget int

	// BundleLogSink, if non-nil, receives one jsonl line per Review() call
	// with the enrichment + resolver telemetry. nil suppresses logging.
	BundleLogSink io.Writer

	// Verbose, if true, emits [verbose] lines to BundleLogSink (or
	// os.Stderr if BundleLogSink is nil).
	Verbose bool
}

// ReviewOptions are per-call parameters.
type ReviewOptions struct {
	// Mode picks the prompt template variant.
	Mode prompt.Mode

	// Focus is an optional free-text focus area to steer the review.
	Focus string

	// Glossary is optional inline glossary content (project-specific terms).
	Glossary string

	// Model overrides the provider's default model name.
	Model string

	// Temperature for sampling. 0 = deterministic. Provider may clamp.
	Temperature float64

	// MaxTokens caps the output. Provider may clamp.
	MaxTokens int
}
