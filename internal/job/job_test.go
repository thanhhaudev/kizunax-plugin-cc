package job

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/llmreviewkit/git"
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
	ws := state.NewWorkspaceDir(dir)
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
	ws := state.NewWorkspaceDir(dir)
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

func TestJob_SerializeNewFields(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	end := now.Add(7 * time.Second)
	j := Job{
		ID:          "T1",
		Kind:        KindReview,
		Status:      StatusCompleted,
		SessionID:   "sess-abc",
		CreatedAt:   now,
		StartedAt:   now,
		CompletedAt: &end,
		DurationMs:  7000,
		Request: Request{
			Mode:     "standard",
			Provider: "openai",
			Model:    "coding/MiniMax-M2.7",
		},
	}
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sessionId":"sess-abc"`, `"durationMs":7000`, `"model":"coding/MiniMax-M2.7"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in JSON: %s", want, data)
		}
	}
}

func makeJobsDir(ws state.WorkspaceDir) error {
	return ensureDir(ws.JobsDir())
}

func ensureDir(p string) error {
	return mkdirAll(p)
}

func TestListBySession_FiltersMatching(t *testing.T) {
	ws := tempWorkspace(t)
	mk := func(id, sess string) {
		j := Job{ID: id, SessionID: sess, Kind: KindReview, Status: StatusCompleted, CreatedAt: time.Now()}
		if err := Save(ws, j); err != nil {
			t.Fatal(err)
		}
	}
	mk("A", "sess-1")
	mk("B", "sess-2")
	mk("C", "sess-1")
	mk("D", "") // legacy job without session

	got, err := ListBySession(ws, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d jobs, want 2", len(got))
	}
	for _, j := range got {
		if j.SessionID != "sess-1" {
			t.Errorf("unexpected sessionID: %s", j.SessionID)
		}
	}
}

func TestJob_DurationAndModel_SerializeRoundtrip(t *testing.T) {
	ws := tempWorkspace(t)
	now := time.Now()
	end := now.Add(2500 * time.Millisecond)
	j := Job{
		ID: "X", Kind: KindReview, Status: StatusCompleted,
		CreatedAt: now, StartedAt: now, CompletedAt: &end,
		DurationMs: 2500,
		Request:    Request{Mode: "standard", Model: "coding/MiniMax-M2.7"},
	}
	if err := Save(ws, j); err != nil {
		t.Fatal(err)
	}
	got, err := Load(ws, "X")
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 2500 {
		t.Errorf("DurationMs lost: got %d", got.DurationMs)
	}
	if got.Request.Model != "coding/MiniMax-M2.7" {
		t.Errorf("Model lost: got %q", got.Request.Model)
	}
}

func TestListBySession_EmptySession_ReturnsAll(t *testing.T) {
	ws := tempWorkspace(t)
	if err := Save(ws, Job{ID: "A", Kind: KindReview, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := Save(ws, Job{ID: "B", SessionID: "sess", Kind: KindReview, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	got, err := ListBySession(ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d, want all 2 when session is empty", len(got))
	}
}
