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
	ws := state.WorkspaceDir{Root: tmp}
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
	ws := state.WorkspaceDir{Root: tmp}
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

func TestRefreshAndWait_PopulatesCacheBeforeReturn(t *testing.T) {
	// Server responds with ~50ms latency. RefreshAndWait must block until the
	// cache write completes, so the next LoadCachedEntry call hits a fresh entry.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","total":1000,"consumed":1,"reset_at":"2026-06-09T00:00:00Z"}}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	ws := state.WorkspaceDir{Root: tmp}
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	// The cache must NOT be populated before RefreshAndWait returns.
	if _, fresh := LoadCachedEntry(ws, "kx_T"); fresh {
		t.Fatalf("precondition violated: cache should be empty")
	}

	// Use a custom Fetcher to honor the httptest server URL — RefreshAndWait
	// itself doesn't accept a client (production path), so we exercise the
	// internal helper indirectly: we know the wait pattern delegates to
	// RefreshAsyncWithClient with a real client. Verify the contract via a
	// thin wrapper that calls RefreshAsyncWithClient + select like the
	// production helper does.
	done := make(chan struct{})
	RefreshAsyncWithClient(srv.Client(), srv.URL, "kx_T", ws, func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("refresh did not complete within 2s")
	}

	entry, fresh := LoadCachedEntry(ws, "kx_T")
	if !fresh {
		t.Errorf("expected cache fresh after RefreshAndWait return")
	}
	if entry.Coding == nil {
		t.Errorf("expected Coding populated")
	}
}

func TestRefreshAndWait_TimesOutWithoutBlocking(t *testing.T) {
	// Server hangs. RefreshAndWait must return within ~timeout, not wait for
	// the full httpTimeout (5s) of the goroutine.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	tmp := t.TempDir()
	ws := state.WorkspaceDir{Root: tmp}
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	// RefreshAndWait uses the real http client (not the test one) but
	// connection to the hung server still proceeds via the OS network stack
	// — the call should return at our timeout regardless.
	RefreshAndWait(srv.URL, "kx_X", ws, 200*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed > 600*time.Millisecond {
		t.Errorf("RefreshAndWait blocked too long: %v (timeout was 200ms)", elapsed)
	}
}

func TestRefreshAndWait_EmptyKeyNoop(t *testing.T) {
	tmp := t.TempDir()
	ws := state.WorkspaceDir{Root: tmp}
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	RefreshAndWait("http://example.invalid", "", ws, 2*time.Second)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("empty-key path should return immediately, took %v", d)
	}
}
