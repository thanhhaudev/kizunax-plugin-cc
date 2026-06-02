package usage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	httpTimeout       = 5 * time.Second
	defaultBarWidth   = 20
	maxConcurrentHTTP = 32
)

// Snapshot is one provider's full pool snapshot.
type Snapshot struct {
	Provider string     // "openai" | "anthropic"
	Usages   []KeyUsage // one per key, in api_keys order
}

// KeyUsage bundles both quotas for a single API key.
type KeyUsage struct {
	KeyHash    string    `json:"-"`
	KeyMask    string    `json:"-"`
	Coding     *Quota    `json:"coding,omitempty"`
	Credits    *Quota    `json:"credits,omitempty"`
	AuthFailed bool      `json:"-"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// Quota is one endpoint's parsed response.
type Quota struct {
	Kind      string    `json:"-"` // "coding" | "credits"
	Plan      string    `json:"plan"`
	Used      int64     `json:"used"`
	Limit     int64     `json:"limit"`
	Remaining int64     `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
	Unlimited bool      `json:"unlimited,omitempty"`
	Err       string    `json:"-"`
}

// Fetcher holds the HTTP client and base URL (scheme+host, no path).
type Fetcher struct {
	Client  *http.Client
	BaseURL string // e.g. "https://kizunax.io"
}

func NewFetcher(baseURL string) *Fetcher {
	return &Fetcher{
		Client:  &http.Client{Timeout: httpTimeout},
		BaseURL: baseURL,
	}
}

// hashKey returns the sha256 hex of an API key for safe use as a cache key.
func hashKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(h[:])
}

// Fetch issues two concurrent requests (Coding + Credits) for one API key.
// AuthFailed is set true ONLY when BOTH endpoints return 401/403; partial
// failures surface as Quota.Err on the failing side.
func (f *Fetcher) Fetch(ctx context.Context, apiKey string) KeyUsage {
	var (
		coding, credits      *Quota
		codingAuth, credAuth bool
		wg                   sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		coding, codingAuth = f.fetchCoding(ctx, apiKey)
	}()
	go func() {
		defer wg.Done()
		credits, credAuth = f.fetchCredits(ctx, apiKey)
	}()
	wg.Wait()

	ku := KeyUsage{
		KeyHash:   hashKey(apiKey),
		KeyMask:   MaskKey(apiKey),
		FetchedAt: time.Now().UTC(),
	}

	if codingAuth && credAuth {
		ku.AuthFailed = true
		return ku
	}

	if codingAuth {
		ku.Coding = &Quota{Kind: "coding", Err: "auth failed (HTTP 401)"}
	} else {
		ku.Coding = coding
	}
	if credAuth {
		ku.Credits = &Quota{Kind: "credits", Err: "auth failed (HTTP 401)"}
	} else {
		ku.Credits = credits
	}
	return ku
}

// MaskKey returns a short display form of an API key, never leaking the full
// secret. Format: prefix (kx_ or first 4) + ellipsis. Exported so CLI callers
// can repopulate KeyUsage.KeyMask after a cache load (cache strips it).
func MaskKey(k string) string {
	if k == "" {
		return "(empty)"
	}
	const ellipsis = "…"
	if len(k) > 3 && k[:3] == "kx_" {
		rest := k[3:]
		if len(rest) > 4 {
			rest = rest[:4]
		}
		return "kx_" + rest + ellipsis
	}
	head := k
	if len(head) > 4 {
		head = head[:4]
	}
	return head + ellipsis
}
