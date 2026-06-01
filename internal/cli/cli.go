package cli

import (
	"fmt"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

const Version = "0.1.0"

func Dispatch(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "review":
		return runReview(args[1:])
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
  kizunax review --working-tree [--verbose]
  kizunax setup [--check | --rebuild]
  kizunax version

Commands:
  review    Review code changes via configured AI provider
  setup     Initialize config (provider, model, API key)
  version   Show version

`, Version)
}
