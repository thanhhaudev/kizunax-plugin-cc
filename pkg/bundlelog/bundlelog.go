// Package bundlelog persists per-review bundle composition records to
// ~/.kizunax/state/{ws-hash}/bundle-log.jsonl for offline pivot during the
// v0.12.4 measurement window. Opt-in via KIZUNAX_BUNDLE_LOG=1 to keep
// steady-state I/O at zero. All errors are swallowed — logging must never
// break a review.
package bundlelog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
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

// Stats is the aggregate counts block. Field names match resolver.ResolveStats(V2)
// + diff.AttachResult for direct jq pivot.
type Stats struct {
	Extracted   int `json:"extracted"`
	Filtered    int `json:"filtered"`
	Resolved    int `json:"resolved"`
	Attached    int `json:"attached"`
	Dropped     int `json:"dropped"`
	BudgetBytes int `json:"budget_bytes"`
	UsedBytes   int `json:"used_bytes"`
	// v0.13.0 additions (omitempty so v0.12.4 entries don't break parsing)
	IndexHits    int    `json:"index_hits,omitempty"`
	IndexMisses  int    `json:"index_misses,omitempty"`
	ResolverPath string `json:"resolver_path,omitempty"`
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

// Append writes one jsonl record. All errors are swallowed — logging must
// never fail a review. If KIZUNAX_DEBUG=1 is also set, write errors are
// echoed to stderr (knob for diagnosing why the log file isn't growing).
//
// Concurrency: multiple processes may Append to the same file. We open with
// O_APPEND and write the full marshaled entry + "\n" as a single syscall.
// POSIX guarantees atomic append for writes ≤4 KiB. Typical entries are
// ~500-2000 bytes.
//
// Rotation: before writing, if the log file is ≥10 MiB, rename it to
// bundle-log.1.jsonl (overwrite existing backup). Rotation race is accepted:
// worst case one backup is lost. This is a measurement tool, not an audit log.
func Append(ws state.WorkspaceDir, entry Entry) {
	if ws.Root == "" {
		return
	}
	path := filepath.Join(ws.Root, LogName)
	backup := filepath.Join(ws.Root, BackupName)

	// Rotate if oversized. Errors here are silent — fall through to write,
	// which will either succeed or also swallow.
	if info, err := os.Stat(path); err == nil && info.Size() >= SizeCapBytes {
		_ = os.Rename(path, backup)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		debugLog("marshal: %v", err)
		return
	}
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		debugLog("open %s: %v", path, err)
		return
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		debugLog("write: %v", err)
	}
}

func debugLog(format string, args ...interface{}) {
	if os.Getenv("KIZUNAX_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[debug] bundlelog: "+format+"\n", args...)
	}
}
