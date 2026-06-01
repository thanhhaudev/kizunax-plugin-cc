package job

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func TestNewID_Format(t *testing.T) {
	id := NewID()
	// Format: YYYYMMDDTHHmmss-XXXXXXXX
	if len(id) != 24 {
		t.Errorf("ID length = %d, want 24 (got %q)", len(id), id)
	}
	if !strings.Contains(id, "-") {
		t.Errorf("ID should contain '-': %q", id)
	}
	parts := strings.SplitN(id, "-", 2)
	if len(parts[0]) != 15 {
		t.Errorf("timestamp part = %q, want length 15", parts[0])
	}
	if len(parts[1]) != 8 {
		t.Errorf("random part = %q, want length 8", parts[1])
	}
}

func TestNewID_Sortable(t *testing.T) {
	ids := make([]string, 5)
	for i := range ids {
		ids[i] = NewID()
		time.Sleep(time.Millisecond)
	}

	sorted := append([]string{}, ids...)
	sort.Strings(sorted)

	// We can't guarantee strict ordering if all 5 happen in the same second,
	// but newest is at the end after sort if sortable, else random.
	// Just check the alphabetical sort puts time-prefixed IDs in time order
	// when timestamps differ.
	first := ids[0]
	last := ids[len(ids)-1]
	if first[:15] != last[:15] && sorted[len(sorted)-1] != last {
		t.Errorf("ID sortability broken: ids[last]=%q not at sorted tail %v",
			last, sorted)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	ws := state.WorkspaceDir{Root: dir}
	if err := makeJobsDir(ws); err != nil {
		t.Fatalf("setup: %v", err)
	}

	original := Job{
		ID:        "20260101T100000-abcdef00",
		Kind:      KindReview,
		Status:    StatusCompleted,
		PID:       12345,
		CreatedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		StartedAt: time.Date(2026, 1, 1, 10, 0, 1, 0, time.UTC),
		Request: Request{
			Mode:   "standard",
			Target: git.Target{Kind: git.TargetWorkingTree},
		},
		LogPath: "/tmp/x.log",
	}

	if err := Save(ws, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(ws, original.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != original.ID || loaded.Status != original.Status || loaded.PID != original.PID {
		t.Errorf("round trip mismatch:\norig:%+v\nload:%+v", original, loaded)
	}
}

func TestList_NewestFirst(t *testing.T) {
	dir := t.TempDir()
	ws := state.WorkspaceDir{Root: dir}
	_ = makeJobsDir(ws)

	older := Job{
		ID: "20260101T100000-aaaaaaaa", Kind: KindReview, Status: StatusCompleted,
		CreatedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
	newer := Job{
		ID: "20260101T120000-bbbbbbbb", Kind: KindReview, Status: StatusCompleted,
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	_ = Save(ws, older)
	_ = Save(ws, newer)

	jobs, err := List(ws)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].ID != newer.ID {
		t.Errorf("expected newest first; got %s before %s", jobs[0].ID, jobs[1].ID)
	}
}

func makeJobsDir(ws state.WorkspaceDir) error {
	return ensureDir(ws.JobsDir())
}

func ensureDir(p string) error {
	return mkdirAll(p)
}
