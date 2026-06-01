package cli

import (
	"fmt"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

const Version = "0.5.1"

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
	case "status":
		return runStatus(args[1:])
	case "result":
		return runResult(args[1:])
	case "cancel":
		return runCancel(args[1:])
	case "internal-execute-job":
		return runInternalExecuteJob(args[1:])
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
  kizunax review              [target-flags] [--focus TEXT] [--background] [--verbose]
  kizunax adversarial-review  [target-flags] [--focus TEXT] [--background] [--verbose]
  kizunax status              [<job-id>]
  kizunax result              <job-id>
  kizunax cancel              <job-id>
  kizunax setup               [--check | --rebuild]
  kizunax version

Target flags (pick at most one; default --working-tree):
  --working-tree              Review uncommitted changes (default)
  --base <ref>                Review branch diff vs <ref>
  --commit <sha>              Review a single commit
  --from <sha> --to <sha>     Review a commit range

Filter:
  --paths a.go,subdir/        Comma-separated path filter

Execution:
  --background                Spawn worker, return job ID immediately
  --provider <name>           Override default: openai | anthropic
  --focus "text"              Optional prompt focus hint
  --verbose                   Print timing + tokens to stderr

Commands:
  review               Standard review
  adversarial-review   Skeptic stance focusing on attack surface and failure modes
  status               List jobs in this workspace, or detail one
  result               Render the result of a finished job
  cancel               SIGTERM a running job's worker
  setup                Initialize config (provider, model, API key)

`, Version)
}
