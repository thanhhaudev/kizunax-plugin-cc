package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
)

func runReview(args []string) error {
	verbose := hasFlag(args, "--verbose")
	_ = hasFlag(args, "--working-tree") // accepted; only target in v0.1

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getwd", "cannot read working directory", err)
	}

	if err := git.EnsureRepo(cwd); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] provider=%s model=%s base_url=%s\n", cfg.Provider, cfg.Model, cfg.BaseURL)
	}

	bundle, err := diff.CollectWorkingTree(cwd)
	if err != nil {
		return err
	}
	if bundle.IsEmpty() {
		fmt.Println("No changes to review. Working tree is clean.")
		return nil
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] diff bytes=%d untracked=%d warnings=%d\n",
			bundle.TotalBytes, len(bundle.Untracked), len(bundle.Warnings))
	}

	pluginRoot, err := resolvePluginRoot()
	if err != nil {
		return err
	}

	p, err := buildProvider(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, err := runner.Run(ctx, pluginRoot, p, bundle, cfg.Model, cfg.Temperature, cfg.MaxTokens)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] tokens in=%d out=%d total=%d\n",
			result.InputTokens, result.OutputTokens, result.TotalTokens)
	}

	out := render.RenderReview(result.Review, bundle, result.TotalTokens)
	fmt.Print(out)
	return nil
}

// resolvePluginRoot finds plugins/kizunax/ relative to the binary or env.
func resolvePluginRoot() (string, error) {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return root, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", xerrors.Internal("exe_path", "cannot determine binary path", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	dir := filepath.Dir(exe)
	candidate := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(candidate, "prompts", "review.md")); err == nil {
		return candidate, nil
	}
	repoRoot := filepath.Join(dir, "..", "..", "..")
	candidate = filepath.Join(repoRoot, "plugins", "kizunax")
	if _, err := os.Stat(filepath.Join(candidate, "prompts", "review.md")); err == nil {
		return candidate, nil
	}
	return "", xerrors.Internal("plugin_root",
		"cannot find plugin root (set CLAUDE_PLUGIN_ROOT)", nil)
}
