package hooks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/runner"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

const (
	stopGateCooldown = 30 * time.Second
	stopGateTimeout  = 30 * time.Second
	maxStopFindings  = 5
)

// StopGateDeps is the seam for injecting diff collection + runner.Run in
// tests. Production passes a value that calls diff.Collect / runner.Run.
type StopGateDeps interface {
	Collect(cwd string) (diff.Bundle, error)
	Run(ctx context.Context, bundle diff.Bundle) (runner.Result, error)
}

// StopGate executes the opt-in review-at-end-of-turn hook. Always returns 0:
// hooks must not block CC.
func StopGate(in io.Reader, out, errOut io.Writer, ws state.WorkspaceDir, cwd string, deps StopGateDeps) int {
	defer recoverSilent(errOut, "stop-gate")

	st, _ := state.LoadStopGate(ws)
	if !st.Enabled {
		return 0
	}

	bundle, err := deps.Collect(cwd)
	if err != nil || bundle.IsEmpty() {
		return 0
	}

	hash := hashBundle(bundle)

	if time.Since(st.LastRunAt) < stopGateCooldown {
		fmt.Fprintf(errOut, "[kizunax-hook stop-gate] skipped: cooldown\n")
		return 0
	}
	if bytes.Equal(hash, st.LastHash) {
		if st.LastVerdictHadHigh && st.LastResult != nil {
			renderCached(out, st.LastResult)
		}
		fmt.Fprintf(errOut, "[kizunax-hook stop-gate] skipped: diff hash unchanged\n")
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), stopGateTimeout)
	defer cancel()

	result, runErr := deps.Run(ctx, bundle)
	if runErr != nil {
		fmt.Fprintf(errOut, "[kizunax-hook stop-gate] error: %v — silent fail\n", runErr)
		// Arm cooldown but DO NOT persist LastHash: a failed review must not
		// poison the unchanged-diff dedupe path. The next Stop event after the
		// cooldown elapses should call Run again on the same diff.
		prev, _ := state.LoadStopGate(ws)
		prev.Enabled = true
		prev.LastRunAt = time.Now()
		_ = state.SaveStopGate(ws, prev)
		return 0
	}

	hadHigh := hasHighSeverity(result.Review)
	cached := summarizeForCache(result.Review)
	_ = state.SaveStopGate(ws, state.StopGateState{
		Enabled: true, LastHash: hash, LastRunAt: time.Now(),
		LastVerdictHadHigh: hadHigh, LastResult: cached,
	})

	if hadHigh {
		fmt.Fprint(out, render.RenderHookWarning(result.Review))
	}
	return 0
}

// hashBundle computes a SHA-256 digest over the diff text and all untracked
// file paths+content. sha256.Hash.Write never returns an error (the stdlib
// implementation always returns nil), so the io.WriteString return values are
// intentionally discarded.
func hashBundle(b diff.Bundle) []byte {
	h := sha256.New()
	_, _ = io.WriteString(h, b.Diff)
	for _, u := range b.Untracked {
		_, _ = io.WriteString(h, u.Path)
		_, _ = io.WriteString(h, u.Content)
	}
	sum := h.Sum(nil)
	return sum[:]
}

func hasHighSeverity(r schema.ReviewResult) bool {
	for _, f := range r.Findings {
		if f.Severity == "high" || f.Severity == "critical" {
			return true
		}
	}
	return false
}

func summarizeForCache(r schema.ReviewResult) *state.CachedVerdict {
	var out []state.CachedFinding
	for _, f := range r.Findings {
		if f.Severity != "high" && f.Severity != "critical" {
			continue
		}
		out = append(out, state.CachedFinding{
			Severity: f.Severity, File: f.File, Line: f.LineStart, Title: f.Title,
		})
		if len(out) >= maxStopFindings {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return &state.CachedVerdict{Findings: out}
}

func renderCached(out io.Writer, cv *state.CachedVerdict) {
	var findings []schema.Finding
	for _, f := range cv.Findings {
		findings = append(findings, schema.Finding{
			Severity: f.Severity, File: f.File, LineStart: f.Line, Title: f.Title,
		})
	}
	fmt.Fprint(out, render.RenderHookWarning(schema.ReviewResult{Findings: findings}))
}
