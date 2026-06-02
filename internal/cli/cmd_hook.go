package cli

import (
	"context"
	"fmt"
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
		hooks.SessionCleanup(os.Stdin, os.Stdout, os.Stderr, ws)
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

func (d *stopGateProductionDeps) Run(ctx context.Context) (runner.Result, error) {
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
	bundle, err := diff.Collect(d.cwd, d.target)
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
