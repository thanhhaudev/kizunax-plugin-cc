package cli

import (
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
)

func TestRunReview_SummaryAndNoSummary_ConflictsAsUserError(t *testing.T) {
	err := runReviewWithMode([]string{"--working-tree", "--summary", "--no-summary"}, prompt.ModeStandard)
	if err == nil {
		t.Fatalf("expected user error for conflicting flags")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "summary") {
		t.Fatalf("error should mention summary flag: %v", err)
	}
}
