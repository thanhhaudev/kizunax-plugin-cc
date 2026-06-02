package usage

import (
	"context"
	"net/http"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// RefreshAsync fires a background goroutine that queries both endpoints for
// usedKey and writes the result to the workspace cache. Errors are swallowed:
// usage data is best-effort. Fire-and-forget; no completion signal.
//
// host is the scheme+host form (e.g. "https://kizunax.io"), NOT the full
// base_url (use DeriveBase to convert).
func RefreshAsync(host, usedKey string, ws state.WorkspaceDir) {
	RefreshAsyncWithClient(nil, host, usedKey, ws, nil)
}

// RefreshAsyncWithClient is RefreshAsync with an injectable *http.Client and
// an optional done callback fired after the cache write attempt. Used by
// tests + by the background worker (Task 16).
func RefreshAsyncWithClient(client *http.Client, host, usedKey string, ws state.WorkspaceDir, done func()) {
	if usedKey == "" {
		if done != nil {
			done()
		}
		return
	}
	go func() {
		defer func() {
			if done != nil {
				done()
			}
			_ = recover() // never crash the parent if cache write panics
		}()
		f := NewFetcher(host)
		if client != nil {
			f.Client = client
		}
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		ku := f.Fetch(ctx, usedKey)
		_ = SaveCache(ws, Snapshot{Usages: []KeyUsage{ku}})
	}()
}
