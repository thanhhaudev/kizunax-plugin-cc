package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/hooks"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runHook(args []string) error {
	if len(args) == 0 {
		return xerrors.User("hook_usage",
			"usage: kizunax hook {session-cleanup|stop-gate}", "")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getcwd", "cannot get cwd", err)
	}
	ws, err := state.Resolve(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[kizunax-hook] state resolve failed: %v\n", err)
		return nil
	}

	switch args[0] {
	case "session-cleanup":
		runHookSessionCleanup(ws, os.Stdin, os.Stdout, os.Stderr)
		return nil

	case "stop-gate":
		deps := newStopGateProductionDeps(cwd)
		hooks.StopGate(os.Stdin, os.Stdout, os.Stderr, ws, cwd, deps)
		return nil

	default:
		return xerrors.User("hook_unknown",
			fmt.Sprintf("unknown hook: %s", args[0]),
			"valid: session-cleanup, stop-gate")
	}
}

// stopGateProductionDeps wires StopGate against the real diff + runner
// pipeline. It uses the same target (working tree) as the spec dictates.
type stopGateProductionDeps struct {
	cwd    string
	target git.Target
}

func newStopGateProductionDeps(cwd string) *stopGateProductionDeps {
	return &stopGateProductionDeps{cwd: cwd, target: git.Target{Kind: git.TargetWorkingTree}}
}

func (d *stopGateProductionDeps) Collect(cwd string) (diff.Bundle, error) {
	return diff.Collect(cwd, d.target)
}

func (d *stopGateProductionDeps) Run(ctx context.Context, bundle diff.Bundle) (runner.Result, error) {
	cfg, err := config.Load("")
	if err != nil {
		return runner.Result{}, err
	}
	pluginRoot, err := ResolvePluginRoot()
	if err != nil {
		return runner.Result{}, err
	}
	p, err := buildProvider(cfg)
	if err != nil {
		return runner.Result{}, err
	}
	return runner.Run(ctx, pluginRoot, p, bundle, runner.Options{
		Mode:        prompt.ModeStandard,
		Focus:       "Quick stop-gate review. Report at most 5 findings, prioritized by severity. Concentrate on correctness bugs and high-impact issues.",
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})
}

// hookInput models the JSON payload Claude Code sends on stdin for hook
// events. Fields we don't need are ignored.
type hookInput struct {
	HookEvent string `json:"hook_event_name"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
}

// runHookSessionCleanup handles the `hook session-cleanup` subcommand. On
// SessionStart it writes KIZUNAX_SESSION_ID to CLAUDE_ENV_FILE so child
// shells inherit the session ID. On SessionEnd (or any other event,
// including missing/unparseable input) it delegates to the existing
// sweep + log-purge body, using the workspace the dispatcher already
// resolved from process cwd — this preserves v0.7 SessionEnd semantics.
//
// Hooks must never break Claude Code sessions: parse failures and env
// write failures are logged to stderr but execution continues.
func runHookSessionCleanup(ws state.WorkspaceDir, stdin io.Reader, stdout, stderr io.Writer) {
	var input hookInput
	if stdin != nil {
		// Best-effort: empty / malformed stdin falls back to zero value.
		data, err := io.ReadAll(stdin)
		if err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &input)
		}
	}

	if input.HookEvent == "SessionStart" {
		envFile := os.Getenv("CLAUDE_ENV_FILE")
		if err := WriteSessionEnv(envFile, input.SessionID); err != nil {
			fmt.Fprintf(stderr, "[kizunax-hook session-cleanup] WriteSessionEnv: %v\n", err)
		}
		// Do NOT sweep on SessionStart — jobs from a previous session
		// may still be valid and the new session has just begun.
		return
	}

	hooks.SessionCleanup(nil, stdout, stderr, ws)
}
