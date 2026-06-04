package runner

import (
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/schema"
)

func TestCanonicalizeFindings_ExactMatchKept(t *testing.T) {
	findings := []schema.Finding{
		{File: "internal/auth/auth.go", Title: "x"},
	}
	paths := []string{"internal/auth/auth.go", "cmd/main.go"}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "internal/auth/auth.go" {
		t.Fatalf("exact-match file should not change, got %q", findings[0].File)
	}
	if len(warnings) != 0 {
		t.Fatalf("no warnings expected, got %v", warnings)
	}
}

func TestCanonicalizeFindings_BasenameUniqueRewritten(t *testing.T) {
	findings := []schema.Finding{
		{File: "auth.go", Title: "race condition"},
	}
	paths := []string{"internal/auth/auth.go", "cmd/main.go"}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "internal/auth/auth.go" {
		t.Fatalf("expected rewrite to full path, got %q", findings[0].File)
	}
	if len(warnings) != 0 {
		t.Fatalf("unambiguous rewrite should not warn, got %v", warnings)
	}
}

func TestCanonicalizeFindings_BasenameAmbiguousKept(t *testing.T) {
	findings := []schema.Finding{
		{File: "auth.go", Title: "race condition"},
	}
	paths := []string{"internal/api/auth.go", "internal/admin/auth.go"}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "auth.go" {
		t.Fatalf("ambiguous basename should stay as-is, got %q", findings[0].File)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "auth.go") {
		t.Fatalf("warning should mention basename: %q", warnings[0])
	}
	if !strings.Contains(warnings[0], "internal/api/auth.go") || !strings.Contains(warnings[0], "internal/admin/auth.go") {
		t.Fatalf("warning should list candidates: %q", warnings[0])
	}
}

func TestCanonicalizeFindings_NoMatchKeptSilently(t *testing.T) {
	findings := []schema.Finding{
		{File: "hallucinated.go", Title: "x"},
	}
	paths := []string{"internal/auth/auth.go"}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "hallucinated.go" {
		t.Fatalf("unknown file should stay as-is (separate concern), got %q", findings[0].File)
	}
	if len(warnings) != 0 {
		t.Fatalf("unknown file should NOT warn (separate concern), got %v", warnings)
	}
}

func TestCanonicalizeFindings_EmptyPathsNoOp(t *testing.T) {
	findings := []schema.Finding{{File: "auth.go", Title: "x"}}
	warnings := canonicalizeFindings(findings, nil)
	if findings[0].File != "auth.go" {
		t.Fatalf("no paths → no-op, got %q", findings[0].File)
	}
	if len(warnings) != 0 {
		t.Fatalf("no warnings expected, got %v", warnings)
	}
}

func TestCanonicalizeFindings_EmptyFileKept(t *testing.T) {
	// An empty file string shouldn't crash or rewrite.
	findings := []schema.Finding{{File: "", Title: "x"}}
	paths := []string{"internal/auth/auth.go"}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "" {
		t.Fatalf("empty file should stay empty, got %q", findings[0].File)
	}
	if len(warnings) != 0 {
		t.Fatalf("no warnings expected, got %v", warnings)
	}
}

func TestCanonicalizeFindings_PreservesUnmodifiedFields(t *testing.T) {
	findings := []schema.Finding{{
		File: "auth.go", Title: "race", Severity: "critical",
		LineStart: 35, LineEnd: 36, Confidence: 0.9,
	}}
	paths := []string{"internal/auth/auth.go"}
	canonicalizeFindings(findings, paths)
	f := findings[0]
	if f.Title != "race" || f.Severity != "critical" ||
		f.LineStart != 35 || f.LineEnd != 36 || f.Confidence != 0.9 {
		t.Fatalf("non-file fields mutated: %+v", f)
	}
}

func TestCanonicalizeFindings_MultipleFindingsMixed(t *testing.T) {
	findings := []schema.Finding{
		{File: "auth.go", Title: "1"},        // unique → rewrite
		{File: "cmd/main.go", Title: "2"},    // exact → keep
		{File: "shared.go", Title: "3"},      // ambiguous → keep + warn
		{File: "nonexistent.go", Title: "4"}, // no match → silent keep
	}
	paths := []string{
		"internal/auth/auth.go",
		"cmd/main.go",
		"pkg/shared.go",
		"util/shared.go",
	}
	warnings := canonicalizeFindings(findings, paths)
	if findings[0].File != "internal/auth/auth.go" {
		t.Fatalf("[0] rewrite failed: %q", findings[0].File)
	}
	if findings[1].File != "cmd/main.go" {
		t.Fatalf("[1] exact match changed: %q", findings[1].File)
	}
	if findings[2].File != "shared.go" {
		t.Fatalf("[2] ambiguous should keep basename: %q", findings[2].File)
	}
	if findings[3].File != "nonexistent.go" {
		t.Fatalf("[3] hallucinated should keep: %q", findings[3].File)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning (for shared.go only), got %v", warnings)
	}
}
