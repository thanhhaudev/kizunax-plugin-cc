package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func loadInput(t *testing.T, name string) schema.ReviewResult {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	var r schema.ReviewResult
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\nrun with -update to create", path, err)
	}
	if string(want) != got {
		t.Errorf("output != %s\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

func TestRenderReview_NeedsAttention_Standard(t *testing.T) {
	r := loadInput(t, "review-needs.json")
	bundle := diff.Bundle{TargetLabel: "commit abc1234"}
	got := RenderReview(r, bundle, 4008, prompt.ModeStandard)
	checkGolden(t, "review-needs.standard.golden.md", got)
}

func TestRenderReview_Approve_Adversarial(t *testing.T) {
	r := loadInput(t, "review-approve.json")
	bundle := diff.Bundle{TargetLabel: "working tree"}
	got := RenderReview(r, bundle, 250, prompt.ModeAdversarial)
	checkGolden(t, "review-approve.adversarial.golden.md", got)
}

func TestRenderReview_WithWarnings(t *testing.T) {
	r := loadInput(t, "review-approve.json")
	bundle := diff.Bundle{
		TargetLabel: "working tree",
		Warnings:    []string{"truncated 2 files >64KB", "skipped binary file: image.png"},
	}
	got := RenderReview(r, bundle, 100, prompt.ModeStandard)
	if !contains(got, "Warnings") {
		t.Error("expected Warnings section in output")
	}
	if !contains(got, "truncated 2 files") {
		t.Error("expected warning text in output")
	}
}

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
	r := loadInput(t, "review-needs.json")
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
