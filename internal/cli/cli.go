package cli

import (
	"fmt"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

const Version = "0.2.0"

func Dispatch(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "review":
		return runReview(args[1:])
	case "adversarial-review":
		return runAdversarialReview(args[1:])
	case "setup":
		return runSetup(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("kizunax %s\n", Version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return xerrors.User("unknown_command",
			fmt.Sprintf("unknown command: %s", args[0]),
			"run 'kizunax help' for usage")
	}
}

func ExitCode(err error) int {
	return xerrors.ExitCode(err)
}

func printUsage() {
	fmt.Printf(`kizunax %s — AI code review for Claude Code

Usage:
  kizunax review              [target-flags] [--focus TEXT] [--verbose]
  kizunax adversarial-review  [target-flags] [--focus TEXT] [--verbose]
  kizunax setup               [--check | --rebuild]
  kizunax version

Target flags (pick at most one; default --working-tree):
  --working-tree              Review uncommitted changes (default)
  --base <ref>                Review branch diff vs <ref>, e.g. --base main
  --commit <sha>              Review a single commit
  --from <sha> --to <sha>     Review a commit range (sha..sha)

Filter (combinable with any target):
  --paths a.go,subdir/        Comma-separated path filter

Other:
  --focus "text"              Optional focus hint (e.g., "auth flow")
  --verbose                   Print timing + token usage to stderr

Commands:
  review               Standard review (correctness + maintainability + security)
  adversarial-review   Skeptic stance focusing on attack surface and failure modes
  setup                Initialize config (provider, model, API key)
  version              Show version

`, Version)
}
