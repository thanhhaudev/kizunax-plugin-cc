package cli

import (
	"io"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

// appendUsageFooter writes a low-quota warning to w based on the cached
// state for usedKey, if any. Cache miss, stale (>60s), or no-low → silent.
// Never returns an error: usage is informational and must not affect callers.
//
// KeyMask is repopulated from usedKey via usage.MaskKey because the cache
// strips it on round-trip (json:"-").
func appendUsageFooter(w io.Writer, ws state.WorkspaceDir, usedKey string) {
	if usedKey == "" {
		return
	}
	entry, fresh := usage.LoadCachedEntry(ws, usedKey)
	if !fresh {
		return
	}
	entry.KeyMask = usage.MaskKey(usedKey)
	writeFooter(w, entry)
}

// appendUsageFooterByHash looks the cache up by a pre-computed sha256 hex hash.
// Used by `kizunax result` where the worker's KeyHash + KeyMask were persisted
// into the job record at run time — avoids calling config.Load (which rotates
// the round-robin counter) just to recover the key identity.
func appendUsageFooterByHash(w io.Writer, ws state.WorkspaceDir, keyHash, keyMask string) {
	if keyHash == "" {
		return
	}
	entry, fresh := usage.LoadCachedEntryByHash(ws, keyHash)
	if !fresh {
		return
	}
	entry.KeyMask = keyMask
	writeFooter(w, entry)
}

func writeFooter(w io.Writer, entry usage.KeyUsage) {
	footer := render.RenderUsageFooter(entry)
	if footer == "" {
		return
	}
	_, _ = io.WriteString(w, footer)
}
