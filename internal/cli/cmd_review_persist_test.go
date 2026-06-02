package cli

import (
	"path/filepath"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// TestForegroundReview_PersistsJobRecord_Roundtrip is a unit-level smoke that
// the persistence path used by runReviewWithMode is wired correctly. Full
// end-to-end coverage requires a real provider; here we exercise the
// state.Resolve + job.Save + ListBySession round trip with a synthetic record
// that mirrors what runReviewWithMode now writes.
func TestForegroundReview_PersistsJobRecord_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KIZUNAX_SESSION_ID", "sess-test")

	ws, err := state.Resolve(filepath.Join(tmp, "repo"))
	if err != nil {
		t.Fatalf("state.Resolve: %v", err)
	}
	rec := job.Job{
		ID:        job.NewID(),
		Kind:      job.KindReview,
		Status:    job.StatusCompleted,
		SessionID: CurrentSessionID(),
		Request: job.Request{
			Mode:     "standard",
			Provider: "openai",
			Model:    "test-model",
		},
	}
	if err := job.Save(ws, rec); err != nil {
		t.Fatalf("job.Save: %v", err)
	}
	got, err := job.ListBySession(ws, "sess-test")
	if err != nil {
		t.Fatalf("job.ListBySession: %v", err)
	}
	if len(got) != 1 || got[0].ID != rec.ID {
		t.Errorf("expected 1 record with id %s; got %+v", rec.ID, got)
	}
	if got[0].Request.Model != "test-model" {
		t.Errorf("expected Model=test-model; got %q", got[0].Request.Model)
	}
}
