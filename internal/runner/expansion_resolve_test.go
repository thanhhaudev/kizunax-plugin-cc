package runner

import (
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// --- parseExpandCSV ---

func TestParseExpandCSV_Basic(t *testing.T) {
	c, td, ts := parseExpandCSV("callers,tests")
	if !c || td || !ts {
		t.Fatalf("want (T,F,T), got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_All(t *testing.T) {
	c, td, ts := parseExpandCSV("all")
	if !c || !td || !ts {
		t.Fatalf("want all-true, got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_None(t *testing.T) {
	c, td, ts := parseExpandCSV("none")
	if c || td || ts {
		t.Fatalf("want all-false, got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_Whitespace(t *testing.T) {
	c, td, ts := parseExpandCSV(" callers , tests ")
	if !c || td || !ts {
		t.Fatalf("whitespace tolerant: want (T,F,T), got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_UnknownIgnored(t *testing.T) {
	c, td, ts := parseExpandCSV("callers,typo")
	if !c || td || ts {
		t.Fatalf("unknown silently dropped: want (T,F,F), got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_Empty(t *testing.T) {
	c, td, ts := parseExpandCSV("")
	if c || td || ts {
		t.Fatalf("empty: want all-false, got (%v,%v,%v)", c, td, ts)
	}
}

func TestParseExpandCSV_CaseInsensitive(t *testing.T) {
	c, td, ts := parseExpandCSV("CALLERS,Tests")
	if !c || td || !ts {
		t.Fatalf("case-insensitive: want (T,F,T), got (%v,%v,%v)", c, td, ts)
	}
}

// --- resolveExpansion ---

func TestResolveExpansion_KillSwitchWins(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "1")
	t.Setenv("KIZUNAX_EXPAND", "all")
	opts := Options{ExpandAll: true}
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Callers: true, TypeDefs: true, Tests: true})

	c, td, ts := resolveExpansion(opts, ws)
	if c || td || ts {
		t.Fatalf("kill switch should override everything: got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_NoExpandPerCall(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	opts := Options{NoExpand: true, ExpandCallers: true}
	c, td, ts := resolveExpansion(opts, state.WorkspaceDir{})
	if c || td || ts {
		t.Fatalf("NoExpand wins over per-flag: got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_ExpandAllShortcut(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	t.Setenv("KIZUNAX_EXPAND", "callers")
	opts := Options{ExpandAll: true}
	c, td, ts := resolveExpansion(opts, state.WorkspaceDir{})
	if !c || !td || !ts {
		t.Fatalf("ExpandAll overrides env: got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_PerFlagOverridesEnv(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	t.Setenv("KIZUNAX_EXPAND", "tests")
	opts := Options{ExpandCallers: true}
	c, td, ts := resolveExpansion(opts, state.WorkspaceDir{})
	if !c || td || ts {
		t.Fatalf("CLI flag wins over env: want (T,F,F), got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_EnvBeatsState(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	t.Setenv("KIZUNAX_EXPAND", "callers")
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Tests: true})

	c, td, ts := resolveExpansion(Options{}, ws)
	if !c || td || ts {
		t.Fatalf("env wins over state: want (T,F,F), got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_StateFileFallback(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	t.Setenv("KIZUNAX_EXPAND", "")
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{TypeDefs: true})

	c, td, ts := resolveExpansion(Options{}, ws)
	if c || !td || ts {
		t.Fatalf("state file: want (F,T,F), got (%v,%v,%v)", c, td, ts)
	}
}

func TestResolveExpansion_AllOffDefault(t *testing.T) {
	t.Setenv("KIZUNAX_DISABLE_EXPAND", "")
	t.Setenv("KIZUNAX_EXPAND", "")
	c, td, ts := resolveExpansion(Options{}, state.WorkspaceDir{})
	if c || td || ts {
		t.Fatalf("default: want (F,F,F), got (%v,%v,%v)", c, td, ts)
	}
}
