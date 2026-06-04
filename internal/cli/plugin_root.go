package cli

import (
	"os"
	"path/filepath"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
)

// ResolvePluginRoot returns the plugins/kizunax/ directory containing
// prompts/ and schemas/. Falls back from CLAUDE_PLUGIN_ROOT env to walking
// up from the binary location.
func ResolvePluginRoot() (string, error) {
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
