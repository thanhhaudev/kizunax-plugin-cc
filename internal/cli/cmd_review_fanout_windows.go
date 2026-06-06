//go:build windows

package cli

import (
	"context"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/llmreviewkit/diff"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/git"
	"github.com/thanhhaudev/llmreviewkit/prompt"
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

// runFanoutReview is not supported on Windows. Fan-out requires process groups
// (Setpgid) which are not available on Windows. Use --strategy=single instead.
func runFanoutReview(_ context.Context, _ runFanoutArgs) error {
	return xerrors.User("fanout_windows",
		"fan-out reviews are not supported on Windows; use --strategy=single", "")
}
