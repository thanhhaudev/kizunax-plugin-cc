package render

import (
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/schema"
)

func TestRenderStatusList_Empty(t *testing.T) {
	got := RenderStatusList(nil)
	if got != "No jobs in this workspace.\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderStatusList_Multiple(t *testing.T) {
	jobs := []job.Job{
		{
			ID: "20260101T120000-aaaaaaaa", Kind: job.KindReview, Status: job.StatusCompleted,
			CreatedAt: time.Now().Add(-2 * time.Minute),
			Request:   job.Request{Target: git.Target{Kind: git.TargetWorkingTree}},
		},
		{
			ID: "20260101T100000-bbbbbbbb", Kind: job.KindAdversarialReview, Status: job.StatusRunning,
			CreatedAt: time.Now().Add(-1 * time.Hour),
			Request:   job.Request{Target: git.Target{Kind: git.TargetCommit, Commit: "abc1234"}},
		},
	}
	got := RenderStatusList(jobs)
	if !contains(got, "completed") || !contains(got, "running") {
		t.Errorf("expected both status icons:\n%s", got)
	}
	if !contains(got, "review") || !contains(got, "adversarial-review") {
		t.Errorf("expected both kinds:\n%s", got)
	}
}

func TestRenderJobDetail_WithResult(t *testing.T) {
	completed := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	r := schema.ReviewResult{
		Verdict: "needs-attention",
		Summary: "test summary",
		Findings: []schema.Finding{
			{Severity: "high", Title: "test finding", File: "foo.go", LineStart: 1, LineEnd: 2, Confidence: 0.9},
		},
	}
	j := job.Job{
		ID:          "20260101T120000-aaaaaaaa",
		Kind:        job.KindReview,
		Status:      job.StatusCompleted,
		PID:         12345,
		CreatedAt:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		StartedAt:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		CompletedAt: &completed,
		Request:     job.Request{Mode: "standard", Target: git.Target{Kind: git.TargetWorkingTree}, Focus: "auth"},
		Result:      &r,
		LogPath:     "/tmp/job.log",
		Tokens:      &job.TokenUsage{Input: 100, Output: 200, Total: 300},
	}
	got := RenderJobDetail(j)
	if !contains(got, "completed") {
		t.Errorf("expected status: %s", got)
	}
	if !contains(got, "Focus") || !contains(got, "auth") {
		t.Error("expected focus to be shown")
	}
	if !contains(got, "100") || !contains(got, "200") {
		t.Error("expected token counts shown")
	}
}

func TestRenderStatusList_IncludesActionsAndDurationColumns(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	end := now.Add(7 * time.Second)
	jobs := []job.Job{
		{
			ID: "20260602T100000-abc", Kind: job.KindReview, Status: job.StatusCompleted,
			CreatedAt: now, StartedAt: now, CompletedAt: &end, DurationMs: 7000,
		},
		{
			ID: "20260602T100100-xyz", Kind: job.KindReview, Status: job.StatusRunning,
			CreatedAt: now,
		},
	}
	out := RenderStatusList(jobs)
	for _, want := range []string{
		"Duration", "Actions",
		"7.0s",
		"`/kizunax:result 20260602T100000-abc`",
		"`/kizunax:cancel 20260602T100100-xyz`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderStatusList missing %q in: %s", want, out)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
