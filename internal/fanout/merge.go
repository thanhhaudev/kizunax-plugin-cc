package fanout

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thanhhaudev/llmreviewkit/schema"
)

// BucketReview pairs a bucket with the review result that came back from its
// worker. F4 builds these by JSON-decoding BucketResult.Stdout into
// schema.ReviewResult.
type BucketReview struct {
	Bucket Bucket
	Result schema.ReviewResult
}

// MergeOptions controls how multiple bucket reviews collapse into one.
type MergeOptions struct {
	// AnnotateFindings, when true, appends "\n\n_Bucket: <prefix>_" to each
	// finding's Body so the rendered output shows where each finding came from.
	// Default false (off — clean unannotated output).
	AnnotateFindings bool

	// BuildTLDR, when true, populates the merged ReviewResult.TLDR with a
	// one-line summary like "Reviewed 4 buckets, 7 findings (2 critical, 5 high)".
	// Default false (off — caller can build their own).
	BuildTLDR bool
}

// Merge collapses N bucket reviews into one ReviewResult:
//
//   - Verdict: "needs-attention" if ANY input has it OR any finding exists;
//     "approve" only when all are "approve" AND no findings.
//   - Summary: concatenated from all inputs (deduped by exact string match).
//   - Findings: union with dedupe by (file, line_start, title) — first
//     occurrence wins. Sorted by severity (critical→high→medium→low),
//     then file, then line_start.
//   - NextSteps: union deduped by exact string match, preserved input order.
//   - TLDR: if opts.BuildTLDR, generated; else empty.
//
// If reviews is empty, returns the zero value of ReviewResult.
func Merge(reviews []BucketReview, opts MergeOptions) schema.ReviewResult {
	if len(reviews) == 0 {
		return schema.ReviewResult{}
	}

	var out schema.ReviewResult

	// Verdict — pessimistic
	verdict := "approve"
	for _, r := range reviews {
		if r.Result.Verdict != "approve" {
			verdict = "needs-attention"
			break
		}
	}

	// Findings — dedupe by (file, line_start, title), first occurrence wins
	seen := map[string]bool{}
	var merged []schema.Finding
	for _, r := range reviews {
		for _, f := range r.Result.Findings {
			key := f.File + ":" + fmt.Sprint(f.LineStart) + ":" + f.Title
			if seen[key] {
				continue
			}
			seen[key] = true
			if opts.AnnotateFindings {
				f.Body = f.Body + "\n\n_Bucket: " + r.Bucket.Prefix + "_"
			}
			merged = append(merged, f)
		}
	}

	// Sort findings: severity → file → line_start
	sevOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sort.SliceStable(merged, func(i, j int) bool {
		if sevOrder[merged[i].Severity] != sevOrder[merged[j].Severity] {
			return sevOrder[merged[i].Severity] < sevOrder[merged[j].Severity]
		}
		if merged[i].File != merged[j].File {
			return merged[i].File < merged[j].File
		}
		return merged[i].LineStart < merged[j].LineStart
	})

	// If we have findings, verdict must be needs-attention regardless of bucket inputs.
	if len(merged) > 0 {
		verdict = "needs-attention"
	}

	// Summary — dedupe by exact string
	summarySeen := map[string]bool{}
	var summaryParts []string
	for _, r := range reviews {
		s := strings.TrimSpace(r.Result.Summary)
		if s == "" || summarySeen[s] {
			continue
		}
		summarySeen[s] = true
		summaryParts = append(summaryParts, s)
	}

	// NextSteps — dedupe preserving order
	stepSeen := map[string]bool{}
	var steps []string
	for _, r := range reviews {
		for _, s := range r.Result.NextSteps {
			s = strings.TrimSpace(s)
			if s == "" || stepSeen[s] {
				continue
			}
			stepSeen[s] = true
			steps = append(steps, s)
		}
	}

	out.Verdict = verdict
	out.Summary = strings.Join(summaryParts, "\n\n")
	out.Findings = merged
	out.NextSteps = steps

	if opts.BuildTLDR {
		out.TLDR = buildTLDR(len(reviews), merged)
	}

	return out
}

func buildTLDR(bucketCount int, findings []schema.Finding) string {
	if len(findings) == 0 {
		return fmt.Sprintf("**TL;DR**: Reviewed %d buckets — no findings.", bucketCount)
	}
	bySev := map[string]int{}
	for _, f := range findings {
		bySev[f.Severity]++
	}
	var parts []string
	for _, s := range []string{"critical", "high", "medium", "low"} {
		if c := bySev[s]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, s))
		}
	}
	return fmt.Sprintf("**TL;DR**: Reviewed %d buckets, %d findings (%s).",
		bucketCount, len(findings), strings.Join(parts, ", "))
}
