package cli

import (
	"fmt"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
)

const Version = "0.23.0"

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
	case "internal-setup-web-worker":
		return runInternalSetupWebWorker(args[1:])
	case "hook":
		return runHook(args[1:])
	case "usage":
		return runUsage(args[1:])
	case "grammars":
		return runGrammars(args[1:])
	case "index":
		return runIndex(args[1:])
	case "expansion":
		return runExpansion(args[1:])
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
  kizunax review              [target-flags] [--focus TEXT] [--quiet] [--verbose]
  kizunax adversarial-review  [target-flags] [--focus TEXT] [--quiet] [--verbose]
  kizunax status              [<job-id-or-prefix>]
  kizunax result              <job-id-or-prefix>
  kizunax cancel              <job-id-or-prefix>
  kizunax setup               [--check | --rebuild]
  kizunax usage               [--provider <name>] [--verbose]
  kizunax version

Target flags (pick at most one; default --working-tree):
  --working-tree              Review uncommitted changes (default)
  --base <ref>                Review branch diff vs <ref>
  --commit <sha>              Review a single commit
  --from <sha> --to <sha>     Review a commit range

Filter:
  --paths a.go,subdir/        Comma-separated path filter

Execution:
  --provider <name>           Override default: openai | anthropic
  --focus "text"              Optional prompt focus hint
  --quiet                     Suppress trailing usage warning footer (for pipe / CI)
  --verbose                   Print timing + tokens to stderr
  --background                Deprecated since v0.9, no-op — async is delegated
                              to Claude Code's Bash(run_in_background:true)

Commands:
  review               Standard review
  adversarial-review   Skeptic stance focusing on attack surface and failure modes
  status               List jobs in current session (or detail one by id/prefix);
                       also sweeps orphaned legacy v0.8 jobs
  result               Render the result of a finished job (accepts id prefix)
  cancel               Cancel a legacy v0.8 running job by id or prefix (v0.9
                       reviews run inline under Claude Code's Bash task, so
                       there is no worker to signal for new runs)
  setup                Initialize config (provider, model, API key)
  usage                Show per-key quota (Coding Plan + Credits) with progress bars
  hook                 Internal — invoked by hooks.json (session-cleanup, stop-gate)

`, Version)
}
