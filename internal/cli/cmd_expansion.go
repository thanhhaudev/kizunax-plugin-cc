package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// runExpansion is the dispatch-entry for `kizunax expansion <sub>`.
// Five subcommands: status | enable <csv> | disable <csv> | set <csv> | reset.
func runExpansion(args []string) error {
	return runExpansionCmd(args, os.Stdout)
}

func runExpansionCmd(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return xerrors.User("expansion_no_subcmd",
			"missing subcommand: status | enable <csv> | disable <csv> | set <csv> | reset",
			"e.g. kizunax expansion status, or /kizunax:expansion enable callers,tests")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getcwd", "cannot resolve cwd", err)
	}
	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("resolve_ws", "cannot resolve workspace", err)
	}

	switch args[0] {
	case "status":
		return runExpansionStatus(ws, cwd, stdout)
	case "enable":
		if len(args) < 2 {
			return xerrors.User("expansion_enable_no_csv",
				"missing strategy list",
				"e.g. kizunax expansion enable callers,tests")
		}
		return runExpansionMutate(ws, args[1], stdout, "enable")
	case "disable":
		if len(args) < 2 {
			return xerrors.User("expansion_disable_no_csv",
				"missing strategy list",
				"e.g. kizunax expansion disable typedefs")
		}
		return runExpansionMutate(ws, args[1], stdout, "disable")
	case "set":
		if len(args) < 2 {
			return xerrors.User("expansion_set_no_csv",
				"missing strategy list",
				"e.g. kizunax expansion set callers,tests")
		}
		return runExpansionMutate(ws, args[1], stdout, "set")
	case "reset":
		return runExpansionReset(ws, stdout)
	default:
		return xerrors.User("expansion_unknown",
			fmt.Sprintf("unknown subcommand: %s", args[0]),
			"available: status | enable | disable | set | reset")
	}
}

func runExpansionStatus(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	st, _ := state.LoadExpansion(ws)
	killSwitch := os.Getenv("KIZUNAX_DISABLE_EXPAND") == "1"
	envCSV := os.Getenv("KIZUNAX_EXPAND")

	fmt.Fprintf(stdout, "Workspace:           %s\n", root)
	fmt.Fprintf(stdout, "State file:          %s\n", ws.ExpansionPath())
	fmt.Fprintf(stdout, "Kill switch env:     %v (KIZUNAX_DISABLE_EXPAND)\n", killSwitch)
	fmt.Fprintf(stdout, "One-shot env:        %q (KIZUNAX_EXPAND)\n", envCSV)
	fmt.Fprintf(stdout, "\nPersistent state (expansion.json):\n")
	fmt.Fprintf(stdout, "  callers:  %s\n", onOff(st.Callers))
	fmt.Fprintf(stdout, "  typedefs: %s\n", onOff(st.TypeDefs))
	fmt.Fprintf(stdout, "  tests:    %s\n", onOff(st.Tests))
	fmt.Fprintln(stdout, "\nFinal precedence: kill switch > CLI flag > env CSV > state file > default OFF.")
	return nil
}

func runExpansionMutate(ws state.WorkspaceDir, csv string, stdout io.Writer, op string) error {
	requested, unknown := parseStrategiesCSV(csv)
	if len(unknown) > 0 {
		return xerrors.User("expansion_unknown_strategy",
			fmt.Sprintf("unknown strategy: %s", strings.Join(unknown, ", ")),
			"available: callers | typedefs | tests | all | none")
	}

	st, _ := state.LoadExpansion(ws)

	switch op {
	case "enable":
		for s := range requested {
			applyStrategy(&st, s, true)
		}
	case "disable":
		for s := range requested {
			applyStrategy(&st, s, false)
		}
	case "set":
		st = state.ExpansionState{}
		for s := range requested {
			applyStrategy(&st, s, true)
		}
	}

	if err := state.SaveExpansion(ws, st); err != nil {
		return xerrors.Internal("save_expansion", "cannot persist state", err)
	}
	fmt.Fprintf(stdout, "Expansion state updated: callers=%s typedefs=%s tests=%s\n",
		onOff(st.Callers), onOff(st.TypeDefs), onOff(st.Tests))
	return nil
}

func runExpansionReset(ws state.WorkspaceDir, stdout io.Writer) error {
	if err := state.DeleteExpansion(ws); err != nil {
		return xerrors.Internal("delete_expansion", "cannot delete state", err)
	}
	fmt.Fprintln(stdout, "Expansion state reset (file deleted). All strategies revert to default OFF.")
	return nil
}

// parseStrategiesCSV returns a set of resolved strategy names and any
// unknown tokens. Recognized: callers, typedefs, tests, all, none
// (case-insensitive, whitespace tolerant). "all" expands to all three;
// "none" is a no-op and contributes no entries.
//
// NOTE: distinct from runner.parseExpandCSV — that returns 3 bools
// (used by the precedence resolver); this returns a set + unknown[]
// (used by the mutator to error on typos).
func parseStrategiesCSV(csv string) (set map[string]bool, unknown []string) {
	set = map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		t := strings.TrimSpace(strings.ToLower(raw))
		switch t {
		case "":
			// skip empty tokens (e.g. trailing comma)
		case "all":
			set["callers"] = true
			set["typedefs"] = true
			set["tests"] = true
		case "none":
			// explicit no-op
		case "callers", "typedefs", "tests":
			set[t] = true
		default:
			unknown = append(unknown, t)
		}
	}
	sort.Strings(unknown)
	return
}

func applyStrategy(st *state.ExpansionState, name string, value bool) {
	switch name {
	case "callers":
		st.Callers = value
	case "typedefs":
		st.TypeDefs = value
	case "tests":
		st.Tests = value
	}
}

func onOff(b bool) string {
	if b {
		return "ON"
	}
	return "off"
}
