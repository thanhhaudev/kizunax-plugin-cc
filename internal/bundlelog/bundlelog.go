// Package bundlelog persists per-review bundle composition records to
// ~/.kizunax/state/{ws-hash}/bundle-log.jsonl for offline pivot during the
// v0.12.4 measurement window. Opt-in via KIZUNAX_BUNDLE_LOG=1 to keep
// steady-state I/O at zero. All errors are swallowed — logging must never
// break a review.
package bundlelog

import (
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
)

const (
	EnvVar       = "KIZUNAX_BUNDLE_LOG"
	LogName      = "bundle-log.jsonl"
	BackupName   = "bundle-log.1.jsonl"
	SizeCapBytes = 10 * 1024 * 1024 // 10 MiB
)

// Entry is one jsonl record (one line per review).
type Entry struct {
	Timestamp string                          `json:"ts"`
	Workspace string                          `json:"ws"`
	DiffFiles int                             `json:"diff_files"`
	Bundle    []diff.ReferencedFileLogEntry   `json:"bundle"`
	Stats     Stats                           `json:"stats"`
}

// Stats is the aggregate counts block. Field names match resolver.ResolveStats
// + diff.AttachResult for direct jq pivot.
type Stats struct {
	Extracted   int `json:"extracted"`
	Filtered    int `json:"filtered"`
	Resolved    int `json:"resolved"`
	Attached    int `json:"attached"`
	Dropped     int `json:"dropped"`
	BudgetBytes int `json:"budget_bytes"`
	UsedBytes   int `json:"used_bytes"`
}

// Enabled returns true iff KIZUNAX_BUNDLE_LOG is set to a truthy value.
// Truthy = "1" or "true" (case-insensitive). Anything else (including "0",
// "false", and unset) returns false.
func Enabled() bool {
	v := os.Getenv(EnvVar)
	switch v {
	case "1", "true", "TRUE", "True":
		return true
	}
	return false
}
