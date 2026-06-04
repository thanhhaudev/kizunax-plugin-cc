package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func TestCmdExpansion_StatusEmptyWorkspace(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	var buf bytes.Buffer
	if err := runExpansionStatus(ws, "/fake/cwd", &buf); err != nil {
		t.Fatalf("status: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "callers:  off") {
		t.Errorf("expected callers off in status, got: %s", out)
	}
	if !strings.Contains(out, "typedefs: off") {
		t.Errorf("expected typedefs off in status, got: %s", out)
	}
	if !strings.Contains(out, "tests:    off") {
		t.Errorf("expected tests off in status, got: %s", out)
	}
}

func TestCmdExpansion_EnableAdditive(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Callers: true})

	var buf bytes.Buffer
	if err := runExpansionMutate(ws, "tests", &buf, "enable"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	got, _ := state.LoadExpansion(ws)
	want := state.ExpansionState{Callers: true, Tests: true}
	if got != want {
		t.Fatalf("enable additive: want %+v, got %+v", want, got)
	}
}

func TestCmdExpansion_DisableSelective(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Callers: true, TypeDefs: true, Tests: true})

	var buf bytes.Buffer
	if err := runExpansionMutate(ws, "typedefs", &buf, "disable"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := state.LoadExpansion(ws)
	want := state.ExpansionState{Callers: true, TypeDefs: false, Tests: true}
	if got != want {
		t.Fatalf("disable selective: want %+v, got %+v", want, got)
	}
}

func TestCmdExpansion_SetReplaces(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Callers: true, Tests: true})

	var buf bytes.Buffer
	if err := runExpansionMutate(ws, "typedefs", &buf, "set"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ := state.LoadExpansion(ws)
	want := state.ExpansionState{Callers: false, TypeDefs: true, Tests: false}
	if got != want {
		t.Fatalf("set replaces: want %+v, got %+v", want, got)
	}
}

func TestCmdExpansion_SetAllShortcut(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	var buf bytes.Buffer
	if err := runExpansionMutate(ws, "all", &buf, "set"); err != nil {
		t.Fatalf("set all: %v", err)
	}
	got, _ := state.LoadExpansion(ws)
	want := state.ExpansionState{Callers: true, TypeDefs: true, Tests: true}
	if got != want {
		t.Fatalf("set all: want %+v, got %+v", want, got)
	}
}

func TestCmdExpansion_ResetDeletesFile(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	_ = state.SaveExpansion(ws, state.ExpansionState{Callers: true})

	var buf bytes.Buffer
	if err := runExpansionReset(ws, &buf); err != nil {
		t.Fatalf("reset: %v", err)
	}
	got, _ := state.LoadExpansion(ws)
	if got != (state.ExpansionState{}) {
		t.Fatalf("reset should yield zero state, got %+v", got)
	}
}

func TestCmdExpansion_UnknownStrategyErrors(t *testing.T) {
	ws := state.NewWorkspaceDir(t.TempDir())
	var buf bytes.Buffer
	err := runExpansionMutate(ws, "bogus", &buf, "enable")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error should mention 'bogus', got: %v", err)
	}
}
