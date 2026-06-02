package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
)

func TestSessionCleanup_MarksOrphanJobs(t *testing.T) {
	ws := makeWS(t)

	j1 := job.Job{
		ID: "20260601T100000-aaaaaaaa", Kind: job.KindReview,
		Status: job.StatusRunning, PID: -1,
		CreatedAt: time.Now(), StartedAt: time.Now(),
	}
	if err := job.Save(ws, j1); err != nil {
		t.Fatal(err)
	}
	j2 := job.Job{
		ID: "20260601T110000-bbbbbbbb", Kind: job.KindReview,
		Status: job.StatusCompleted, PID: -1,
		CreatedAt: time.Now(), StartedAt: time.Now(),
	}
	if err := job.Save(ws, j2); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	rc := SessionCleanup(strings.NewReader(""), &bytes.Buffer{}, &stderr, ws)
	if rc != 0 {
		t.Errorf("rc: got %d want 0", rc)
	}

	got1, _ := job.Load(ws, j1.ID)
	if got1.Status != job.StatusFailed && got1.Status != job.StatusCancelled {
		t.Errorf("j1 status: got %s, want failed or cancelled", got1.Status)
	}
	got2, _ := job.Load(ws, j2.ID)
	if got2.Status != job.StatusCompleted {
		t.Errorf("j2 status changed: got %s want completed", got2.Status)
	}
}

func TestSessionCleanup_DeletesOldLogs(t *testing.T) {
	ws := makeWS(t)
	old := filepath.Join(ws.JobsDir(), "old.log")
	if err := os.WriteFile(old, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(old, tenDaysAgo, tenDaysAgo)

	rc := SessionCleanup(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws)
	if rc != 0 {
		t.Errorf("rc: got %d", rc)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old log not deleted")
	}
}

func TestSessionCleanup_Idempotent(t *testing.T) {
	ws := makeWS(t)
	j := job.Job{
		ID: "20260601T100000-cccccccc", Kind: job.KindReview,
		Status: job.StatusRunning, PID: -1,
		CreatedAt: time.Now(), StartedAt: time.Now(),
	}
	_ = job.Save(ws, j)

	rc1 := SessionCleanup(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws)
	rc2 := SessionCleanup(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws)
	if rc1 != 0 || rc2 != 0 {
		t.Errorf("rc: got %d, %d", rc1, rc2)
	}

	got, _ := job.Load(ws, j.ID)
	if got.Status == job.StatusRunning {
		t.Errorf("expected non-running after cleanup, got running")
	}
}

func TestSessionCleanup_EmptyWorkspaceNoError(t *testing.T) {
	ws := makeWS(t)
	rc := SessionCleanup(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, ws)
	if rc != 0 {
		t.Errorf("rc: got %d", rc)
	}
}
