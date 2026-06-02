package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
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
