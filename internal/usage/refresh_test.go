package usage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func TestRefreshAsync_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","total":1000,"consumed":1,"reset_at":"2026-06-09T00:00:00Z"}}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	RefreshAsyncWithClient(srv.Client(), srv.URL, "kx_TEST", ws, func() { wg.Done() })
	wg.Wait()

	entry, fresh := LoadCachedEntry(ws, "kx_TEST")
	if !fresh {
		t.Errorf("entry should be fresh")
	}
	if entry.Coding == nil || entry.Credits == nil {
		t.Errorf("both quotas should be present: coding=%v credits=%v", entry.Coding, entry.Credits)
	}
}

func TestRefreshAsync_ContextCancelled(t *testing.T) {
	// Server hangs until client closes the connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	RefreshAsyncWithClient(srv.Client(), srv.URL, "kx_X", ws, func() { wg.Done() })

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Errorf("RefreshAsync did not finish within client timeout window")
	}
}

// TestRefreshAsyncWithClient_DoneCallbackBlocksUntilCacheWritten locks in the
// wait pattern that both `RefreshAndWait` (production) and the worker rely on:
// the done callback fires only after SaveCache has run, so callers that block
// on `done` see a populated cache. RefreshAndWait builds its own *http.Client
// and is therefore not directly addressable by httptest; this test exercises
// the underlying primitive it wraps.
func TestRefreshAsyncWithClient_DoneCallbackBlocksUntilCacheWritten(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","total":1000,"consumed":1,"reset_at":"2026-06-09T00:00:00Z"}}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, fresh := LoadCachedEntry(ws, "kx_T"); fresh {
		t.Fatalf("precondition violated: cache should be empty")
	}

	done := make(chan struct{})
	RefreshAsyncWithClient(srv.Client(), srv.URL, "kx_T", ws, func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("refresh did not complete within 2s")
	}

	entry, fresh := LoadCachedEntry(ws, "kx_T")
	if !fresh {
		t.Errorf("expected cache fresh after done fired")
	}
	if entry.Coding == nil {
		t.Errorf("expected Coding populated")
	}
}

// Timeout semantics of RefreshAndWait are mechanical (literal select + time.After)
// and not race-detector-safe to assert from a test because the inner goroutine
// outlives the wait window and races t.TempDir cleanup. Verified by inspection
// of the 4-line wrapper instead.

func TestRefreshAndWait_EmptyKeyNoop(t *testing.T) {
	tmp := t.TempDir()
	ws := state.NewWorkspaceDir(tmp)
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	RefreshAndWait("http://example.invalid", "", ws, 2*time.Second)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("empty-key path should return immediately, took %v", d)
	}
}
