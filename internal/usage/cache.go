// internal/usage/cache.go
package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

const cacheTTL = 60 * time.Second

// cacheFile is the on-disk shape.
type cacheFile struct {
	Entries map[string]KeyUsage `json:"entries"`
}

func cachePath(ws state.WorkspaceDir) string {
	return filepath.Join(ws.Root, "usage.json")
}

// LoadCache reads the cache map. Missing or corrupt files yield an empty map
// without error (caller treats as cache miss).
func LoadCache(ws state.WorkspaceDir) (map[string]KeyUsage, error) {
	data, err := os.ReadFile(cachePath(ws))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]KeyUsage{}, nil
		}
		return map[string]KeyUsage{}, nil
	}
	var f cacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		return map[string]KeyUsage{}, nil
	}
	if f.Entries == nil {
		f.Entries = map[string]KeyUsage{}
	}
	return f.Entries, nil
}

// SaveCache writes the snapshot, omitting:
//   - quotas with a non-empty Err
//   - keys with AuthFailed=true (both endpoints failed auth)
//   - entries where neither Coding nor Credits survived filtering
//
// Existing cache entries for unrelated keys are PRESERVED across writes.
func SaveCache(ws state.WorkspaceDir, s Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(cachePath(ws)), 0o700); err != nil {
		return err
	}
	existing, _ := LoadCache(ws)

	for _, ku := range s.Usages {
		if ku.AuthFailed || ku.KeyHash == "" {
			continue
		}
		clean := KeyUsage{
			KeyHash:   ku.KeyHash,
			KeyMask:   ku.KeyMask,
			FetchedAt: ku.FetchedAt,
		}
		if ku.Coding != nil && ku.Coding.Err == "" {
			clean.Coding = ku.Coding
		}
		if ku.Credits != nil && ku.Credits.Err == "" {
			clean.Credits = ku.Credits
		}
		if clean.Coding == nil && clean.Credits == nil {
			continue
		}
		existing[ku.KeyHash] = clean
	}

	out := cacheFile{Entries: existing}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteAtomic(cachePath(ws), data, 0o600)
}

// LoadCachedEntry returns the cached KeyUsage for apiKey and a boolean
// indicating freshness (FetchedAt within cacheTTL).
func LoadCachedEntry(ws state.WorkspaceDir, apiKey string) (KeyUsage, bool) {
	cache, _ := LoadCache(ws)
	entry, ok := cache[hashKey(apiKey)]
	if !ok {
		return KeyUsage{}, false
	}
	if time.Since(entry.FetchedAt) > cacheTTL {
		return entry, false
	}
	return entry, true
}
