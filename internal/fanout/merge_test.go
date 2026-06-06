package fanout

import (
	"strings"
	"testing"

	"github.com/thanhhaudev/llmreviewkit/schema"
)

func TestMerge_Empty(t *testing.T) {
	got := Merge(nil, MergeOptions{})
	if got.Verdict != "" || len(got.Findings) != 0 {
		t.Errorf("empty input: got %+v, want zero value", got)
	}
}

func TestMerge_SingleBucket(t *testing.T) {
	input := []BucketReview{{
		Bucket: Bucket{Prefix: "api"},
		Result: schema.ReviewResult{
			Verdict: "needs-attention",
			Summary: "Found 1 issue",
			Findings: []schema.Finding{{
				Severity: "high", File: "api/main.go", LineStart: 10, Title: "Bug",
			}},
		},
	}}
	got := Merge(input, MergeOptions{})
	if got.Verdict != "needs-attention" {
		t.Errorf("verdict: got %q", got.Verdict)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings: got %d, want 1", len(got.Findings))
	}
	if got.Findings[0].File != "api/main.go" {
		t.Errorf("finding file wrong: %+v", got.Findings[0])
	}
}

func TestMerge_DedupesByFileLineTitle(t *testing.T) {
	same := schema.Finding{
		Severity: "high", File: "shared.go", LineStart: 5, Title: "Duplicate",
	}
	input := []BucketReview{
		{Bucket: Bucket{Prefix: "api"}, Result: schema.ReviewResult{Verdict: "needs-attention", Findings: []schema.Finding{same}}},
		{Bucket: Bucket{Prefix: "web"}, Result: schema.ReviewResult{Verdict: "needs-attention", Findings: []schema.Finding{same}}},
	}
	got := Merge(input, MergeOptions{})
	if len(got.Findings) != 1 {
		t.Errorf("dedupe failed: got %d findings, want 1", len(got.Findings))
	}
}

func TestMerge_SortsBySeverityThenFileThenLine(t *testing.T) {
	input := []BucketReview{{
		Bucket: Bucket{Prefix: "."},
		Result: schema.ReviewResult{
			Verdict: "needs-attention",
			Findings: []schema.Finding{
				{Severity: "low", File: "z.go", LineStart: 1, Title: "A"},
				{Severity: "critical", File: "b.go", LineStart: 1, Title: "B"},
				{Severity: "critical", File: "a.go", LineStart: 5, Title: "C"},
				{Severity: "critical", File: "a.go", LineStart: 1, Title: "D"},
				{Severity: "high", File: "m.go", LineStart: 1, Title: "E"},
			},
		},
	}}
	got := Merge(input, MergeOptions{})
	if len(got.Findings) != 5 {
		t.Fatalf("got %d findings, want 5", len(got.Findings))
	}
	expected := []string{"D", "C", "B", "E", "A"} // critical(a.go:1)→critical(a.go:5)→critical(b.go:1)→high→low
	for i, want := range expected {
		if got.Findings[i].Title != want {
			t.Errorf("findings[%d]: got %q, want %q (full: %+v)", i, got.Findings[i].Title, want, got.Findings[i])
		}
	}
}

func TestMerge_VerdictApproveOnlyWhenAllApproveAndNoFindings(t *testing.T) {
	cases := []struct {
		name        string
		verdicts    []string
		findings    bool
		wantVerdict string
	}{
		{"all approve no findings", []string{"approve", "approve"}, false, "approve"},
		{"mixed verdict no findings", []string{"approve", "needs-attention"}, false, "needs-attention"},
		{"all approve with findings", []string{"approve", "approve"}, true, "needs-attention"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input []BucketReview
			for i, v := range tc.verdicts {
				r := schema.ReviewResult{Verdict: v}
				if tc.findings && i == 0 {
					r.Findings = []schema.Finding{{Severity: "high", File: "x.go", LineStart: 1, Title: "F"}}
				}
				input = append(input, BucketReview{
					Bucket: Bucket{Prefix: "b"},
					Result: r,
				})
			}
			got := Merge(input, MergeOptions{})
			if got.Verdict != tc.wantVerdict {
				t.Errorf("verdict: got %q, want %q", got.Verdict, tc.wantVerdict)
			}
		})
	}
}

func TestMerge_AnnotateFindings(t *testing.T) {
	input := []BucketReview{{
		Bucket: Bucket{Prefix: "api/cmd"},
		Result: schema.ReviewResult{
			Verdict: "needs-attention",
			Findings: []schema.Finding{{
				Severity: "high", File: "x.go", LineStart: 1, Title: "T", Body: "original",
			}},
		},
	}}
	got := Merge(input, MergeOptions{AnnotateFindings: true})
	if !strings.Contains(got.Findings[0].Body, "Bucket: api/cmd") {
		t.Errorf("annotation missing: %q", got.Findings[0].Body)
	}
	if !strings.Contains(got.Findings[0].Body, "original") {
		t.Errorf("original body lost: %q", got.Findings[0].Body)
	}
}

func TestMerge_BuildTLDR(t *testing.T) {
	input := []BucketReview{{
		Bucket: Bucket{Prefix: "api"},
		Result: schema.ReviewResult{
			Verdict: "needs-attention",
			Findings: []schema.Finding{
				{Severity: "critical", File: "a.go", LineStart: 1, Title: "A"},
				{Severity: "critical", File: "b.go", LineStart: 2, Title: "B"},
				{Severity: "high", File: "c.go", LineStart: 3, Title: "C"},
			},
		},
	}}
	got := Merge(input, MergeOptions{BuildTLDR: true})
	if !strings.Contains(got.TLDR, "3 findings") {
		t.Errorf("TLDR missing total: %q", got.TLDR)
	}
	if !strings.Contains(got.TLDR, "2 critical") {
		t.Errorf("TLDR missing critical count: %q", got.TLDR)
	}
	if !strings.Contains(got.TLDR, "1 high") {
		t.Errorf("TLDR missing high count: %q", got.TLDR)
	}
}

func TestMerge_BuildTLDR_NoFindings(t *testing.T) {
	input := []BucketReview{
		{Bucket: Bucket{Prefix: "api"}, Result: schema.ReviewResult{Verdict: "approve"}},
		{Bucket: Bucket{Prefix: "web"}, Result: schema.ReviewResult{Verdict: "approve"}},
	}
	got := Merge(input, MergeOptions{BuildTLDR: true})
	if !strings.Contains(got.TLDR, "2 buckets") {
		t.Errorf("TLDR missing bucket count: %q", got.TLDR)
	}
	if !strings.Contains(got.TLDR, "no findings") {
		t.Errorf("TLDR should mention no findings: %q", got.TLDR)
	}
}

func TestMerge_SummaryDedupe(t *testing.T) {
	input := []BucketReview{
		{Bucket: Bucket{Prefix: "a"}, Result: schema.ReviewResult{Summary: "shared text"}},
		{Bucket: Bucket{Prefix: "b"}, Result: schema.ReviewResult{Summary: "shared text"}},
		{Bucket: Bucket{Prefix: "c"}, Result: schema.ReviewResult{Summary: "unique to c"}},
	}
	got := Merge(input, MergeOptions{})
	count := strings.Count(got.Summary, "shared text")
	if count != 1 {
		t.Errorf("summary deduped wrong: %d occurrences of shared text in %q", count, got.Summary)
	}
	if !strings.Contains(got.Summary, "unique to c") {
		t.Errorf("unique summary lost: %q", got.Summary)
	}
}

func TestMerge_NextStepsDedupe(t *testing.T) {
	input := []BucketReview{
		{Bucket: Bucket{Prefix: "a"}, Result: schema.ReviewResult{NextSteps: []string{"step1", "step2"}}},
		{Bucket: Bucket{Prefix: "b"}, Result: schema.ReviewResult{NextSteps: []string{"step2", "step3"}}},
	}
	got := Merge(input, MergeOptions{})
	if len(got.NextSteps) != 3 {
		t.Errorf("nextsteps: got %d, want 3 (step1, step2, step3); raw=%v", len(got.NextSteps), got.NextSteps)
	}
}
