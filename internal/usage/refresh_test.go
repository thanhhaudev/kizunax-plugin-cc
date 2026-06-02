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
