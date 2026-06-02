package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

// TestAppendUsageFooterIfNotQuiet_QuietSuppresses verifies the --quiet flag
// suppresses the trailing usage warning footer used by /kizunax:review,
// even when a fresh low-quota entry exists in the cache.
func TestAppendUsageFooterIfNotQuiet_QuietSuppresses(t *testing.T) {
	ws := makeWS(t)
	// Seed a fresh-low entry so a non-quiet call WOULD write a footer.
	// This proves the suppression is specifically caused by quiet=true.
	seedCache(t, ws, "kx_QUIET",
		&usage.Quota{Kind: "coding", Plan: "free", Used: 4996, Limit: 5000, Remaining: 4, ResetAt: time.Now().Add(3 * time.Minute)},
		nil,
		5*time.Second,
	)

	var buf bytes.Buffer
	appendUsageFooterIfNotQuiet(&buf, true /*quiet*/, ws, "kx_QUIET")
	if buf.Len() != 0 {
		t.Errorf("expected no footer under --quiet, got: %q", buf.String())
	}
}

// TestAppendUsageFooterIfNotQuiet_NotQuietDelegates verifies that when
// quiet=false the wrapper still writes the warning for a fresh low entry,
// i.e. it delegates to appendUsageFooter normally.
func TestAppendUsageFooterIfNotQuiet_NotQuietDelegates(t *testing.T) {
	ws := makeWS(t)
	seedCache(t, ws, "kx_LOUD",
		&usage.Quota{Kind: "coding", Plan: "free", Used: 4996, Limit: 5000, Remaining: 4, ResetAt: time.Now().Add(3 * time.Minute)},
		nil,
		5*time.Second,
	)

	var buf bytes.Buffer
	appendUsageFooterIfNotQuiet(&buf, false /*quiet*/, ws, "kx_LOUD")
	if buf.Len() == 0 {
		t.Errorf("expected footer when quiet=false and entry is low, got empty")
	}
}
