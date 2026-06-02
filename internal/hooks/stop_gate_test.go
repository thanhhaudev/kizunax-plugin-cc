package hooks

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

type fakeDeps struct {
	bundle    diff.Bundle
	bundleErr error
	runResult runner.Result
	runErr    error
	runCalls  int
}

func (f *fakeDeps) Collect(cwd string) (diff.Bundle, error) {
	return f.bundle, f.bundleErr
}
func (f *fakeDeps) Run(ctx context.Context, bundle diff.Bundle) (runner.Result, error) {
	f.runCalls++
	return f.runResult, f.runErr
}

func TestStopGate_DisabledShortCircuits(t *testing.T) {
	ws := makeWS(t)
	deps := &fakeDeps{}
	var stdout, stderr bytes.Buffer
	rc := StopGate(strings.NewReader(""), &stdout, &stderr, ws, ".", deps)
	if rc != 0 {
		t.Errorf("rc: got %d", rc)
	}
	if deps.runCalls != 0 {
		t.Errorf("Run called %d times, want 0", deps.runCalls)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty, got: %s", stdout.String())
	}
}

func TestStopGate_EmptyDiffSkips(t *testing.T) {
	ws := makeWS(t)
	_ = state.SaveStopGate(ws, state.StopGateState{Enabled: true})
	deps := &fakeDeps{bundle: diff.Bundle{}} // IsEmpty()=true
	rc := StopGate(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 || deps.runCalls != 0 {
		t.Errorf("expected no Run on empty diff")
	}
}

func TestStopGate_CooldownSkips(t *testing.T) {
	ws := makeWS(t)
	_ = state.SaveStopGate(ws, state.StopGateState{
		Enabled:   true,
		LastRunAt: time.Now().Add(-10 * time.Second),
	})
	deps := &fakeDeps{bundle: diff.Bundle{Diff: "x"}}
	rc := StopGate(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 || deps.runCalls != 0 {
		t.Errorf("cooldown should skip Run")
	}
}

func TestStopGate_HashMatchSkipsWithCachedRender(t *testing.T) {
	ws := makeWS(t)
	bundle := diff.Bundle{Diff: "hello"}
	hash := hashBundle(bundle)
	_ = state.SaveStopGate(ws, state.StopGateState{
		Enabled:            true,
		LastRunAt:          time.Now().Add(-2 * time.Minute),
		LastHash:           hash,
		LastVerdictHadHigh: true,
		LastResult: &state.CachedVerdict{
			Findings: []state.CachedFinding{
				{Severity: "high", File: "x.go", Line: 1, Title: "cached"},
			},
		},
	})
	deps := &fakeDeps{bundle: bundle}
	var stdout bytes.Buffer
	rc := StopGate(strings.NewReader(""), &stdout, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 || deps.runCalls != 0 {
		t.Errorf("hash match should skip Run")
	}
	if !strings.Contains(stdout.String(), "cached") {
		t.Errorf("expected cached warning rendered, got:\n%s", stdout.String())
	}
}

func TestStopGate_HighFindingRendersWarning(t *testing.T) {
	ws := makeWS(t)
	_ = state.SaveStopGate(ws, state.StopGateState{Enabled: true})
	deps := &fakeDeps{
		bundle: diff.Bundle{Diff: "x"},
		runResult: runner.Result{
			Review: schema.ReviewResult{
				Findings: []schema.Finding{
					{Severity: "high", File: "a.go", LineStart: 10, Title: "bug"},
				},
			},
		},
	}
	var stdout bytes.Buffer
	rc := StopGate(strings.NewReader(""), &stdout, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 || deps.runCalls != 1 {
		t.Errorf("expected 1 Run call, got %d", deps.runCalls)
	}
	if !strings.Contains(stdout.String(), "stop-gate") {
		t.Errorf("expected warning, got:\n%s", stdout.String())
	}
	got, _ := state.LoadStopGate(ws)
	if !got.LastVerdictHadHigh {
		t.Errorf("LastVerdictHadHigh not persisted")
	}
}

func TestStopGate_NoHighSilent(t *testing.T) {
	ws := makeWS(t)
	_ = state.SaveStopGate(ws, state.StopGateState{Enabled: true})
	deps := &fakeDeps{
		bundle: diff.Bundle{Diff: "x"},
		runResult: runner.Result{
			Review: schema.ReviewResult{
				Findings: []schema.Finding{
					{Severity: "low", File: "a.go", LineStart: 1, Title: "minor"},
				},
			},
		},
	}
	var stdout bytes.Buffer
	rc := StopGate(strings.NewReader(""), &stdout, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 {
		t.Errorf("rc: got %d", rc)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected silent on no high findings, got: %s", stdout.String())
	}
}

func TestStopGate_RunErrorSilentAndCooldownArmed(t *testing.T) {
	ws := makeWS(t)
	_ = state.SaveStopGate(ws, state.StopGateState{Enabled: true})
	deps := &fakeDeps{
		bundle: diff.Bundle{Diff: "x"},
		runErr: errors.New("boom"),
	}
	rc := StopGate(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws, ".", deps)
	if rc != 0 {
		t.Errorf("rc: got %d (must always be 0)", rc)
	}
	got, _ := state.LoadStopGate(ws)
	if got.LastRunAt.IsZero() {
		t.Errorf("LastRunAt should be armed even on error, to honor cooldown")
	}
	if len(got.LastHash) != 0 {
		t.Errorf("LastHash must not be persisted on error (would poison dedupe), got len=%d", len(got.LastHash))
	}
}

// Regression for adversarial-review finding: a transient Run error must not
// poison the unchanged-diff dedupe cache. After cooldown elapses, the same
// dirty diff must trigger Run again rather than silently skipping forever.
func TestStopGate_ErrorThenSameHashRetriesAfterCooldown(t *testing.T) {
	ws := makeWS(t)
	// Pre-arm enabled state with NO previous hash/result.
	_ = state.SaveStopGate(ws, state.StopGateState{Enabled: true})

	bundle := diff.Bundle{Diff: "x"}

	// First invocation: Run errors.
	failingDeps := &fakeDeps{bundle: bundle, runErr: errors.New("boom")}
	_ = StopGate(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws, ".", failingDeps)
	if failingDeps.runCalls != 1 {
		t.Fatalf("first Run call count: got %d want 1", failingDeps.runCalls)
	}

	// Roll LastRunAt back past the cooldown window to simulate elapsed time.
	st, _ := state.LoadStopGate(ws)
	st.LastRunAt = time.Now().Add(-2 * time.Minute)
	_ = state.SaveStopGate(ws, st)

	// Second invocation, same bundle: must call Run again.
	retryDeps := &fakeDeps{
		bundle: bundle,
		runResult: runner.Result{
			Review: schema.ReviewResult{
				Findings: []schema.Finding{
					{Severity: "high", File: "a.go", LineStart: 10, Title: "real bug"},
				},
			},
		},
	}
	var stdout bytes.Buffer
	_ = StopGate(strings.NewReader(""), &stdout, &bytes.Buffer{}, ws, ".", retryDeps)
	if retryDeps.runCalls != 1 {
		t.Errorf("retry Run call count: got %d want 1 (failed review must not poison dedupe)", retryDeps.runCalls)
	}
	if !strings.Contains(stdout.String(), "stop-gate") {
		t.Errorf("expected warning rendered after successful retry, got:\n%s", stdout.String())
	}
}
