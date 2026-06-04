package runner

import (
	"os"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// parseExpandCSV parses a comma-separated list of strategy names and
// returns the three per-strategy bools. Recognized tokens (case-
// insensitive, whitespace tolerant):
//
//	"callers", "typedefs", "tests" — set the corresponding bool true
//	"all"                          — short-circuit, returns (true, true, true)
//	"none", "" (empty token)       — no-op (no bool set)
//
// Unknown tokens are silently ignored for forward compatibility.
func parseExpandCSV(csv string) (callers, typedefs, tests bool) {
	for _, raw := range strings.Split(csv, ",") {
		t := strings.TrimSpace(strings.ToLower(raw))
		switch t {
		case "all":
			return true, true, true
		case "none", "":
			// no-op
		case "callers":
			callers = true
		case "typedefs":
			typedefs = true
		case "tests":
			tests = true
		}
	}
	return
}

// resolveExpansion walks the precedence stack (kill switch → CLI
// flag → env → state file → default) and returns the final per-
// strategy bools that will be passed to engine.Config.
//
// Layers (earliest wins):
//
//	1. KIZUNAX_DISABLE_EXPAND=1 env — kill switch (all-off)
//	2. opts.NoExpand                — per-call kill switch (all-off)
//	3. opts.ExpandAll               — per-call shortcut (all-on)
//	4. opts.ExpandCallers / opts.ExpandTypeDefs / opts.ExpandTests
//	   (if any of the three is set, return those exact values; the
//	   CLI is authoritative for this invocation)
//	5. KIZUNAX_EXPAND env CSV
//	6. State file expansion.json (per-workspace persistent baseline)
//	7. Default (all-off)
func resolveExpansion(opts Options, ws state.WorkspaceDir) (callers, typedefs, tests bool) {
	if os.Getenv("KIZUNAX_DISABLE_EXPAND") == "1" {
		return false, false, false
	}
	if opts.NoExpand {
		return false, false, false
	}
	if opts.ExpandAll {
		return true, true, true
	}
	if opts.ExpandCallers || opts.ExpandTypeDefs || opts.ExpandTests {
		return opts.ExpandCallers, opts.ExpandTypeDefs, opts.ExpandTests
	}
	if csv := os.Getenv("KIZUNAX_EXPAND"); csv != "" {
		return parseExpandCSV(csv)
	}
	if ws.Root != "" {
		if st, err := state.LoadExpansion(ws); err == nil {
			return st.Callers, st.TypeDefs, st.Tests
		}
	}
	return false, false, false
}
