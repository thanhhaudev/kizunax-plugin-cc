package job

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/thanhhaudev/llmreviewkit/git"
	"github.com/thanhhaudev/llmreviewkit/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

type Kind string

const (
	KindReview            Kind = "review"
	KindAdversarialReview Kind = "adversarial-review"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Request captures everything needed to reproduce a review run.
type Request struct {
	Mode     string     `json:"mode"`
	Target   git.Target `json:"target"`
	Focus    string     `json:"focus,omitempty"`
	Provider string     `json:"provider,omitempty"` // resolved provider name (openai|anthropic)
	Model    string     `json:"model,omitempty"`    // pinned at spawn; worker uses this not config.Model
	// KeyHash is the sha256 hex of the API key picked by the worker. Set after
	// the worker resolves config so `kizunax result` can read the usage cache
	// for the exact key that produced the review — config.Load rotates, so a
	// later Load may return a different key.
	KeyHash string `json:"key_hash,omitempty"`
	// KeyMask is the display-safe mask of the same key (e.g. "kx_AbCd…").
	// Persisted because the raw key is never stored.
	KeyMask string `json:"key_mask,omitempty"`

	// v0.11+ helper TL;DR fields. Persisted at spawn so future background
	// workers stay consistent across config changes / key rotation. The
	// raw helper key is NEVER stored — only hash + mask, mirroring the
	// KeyHash/KeyMask pattern above.
	Summary       bool   `json:"summary,omitempty"`
	NoSummary     bool   `json:"no_summary,omitempty"`
	HelperBaseURL string `json:"helper_base_url,omitempty"`
	HelperModel   string `json:"helper_model,omitempty"`
	HelperKeyHash string `json:"helper_key_hash,omitempty"`
	HelperKeyMask string `json:"helper_key_mask,omitempty"`

	// v0.12+: paths of referenced files included in the review prompt.
	// Persisted for debugging via `kizunax result <id>`. CONTENT NOT STORED
	// — privacy + bloat (same reasoning as glossary v0.11).
	ReferencedFilePaths []string `json:"referenced_file_paths,omitempty"`
}

type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

type Job struct {
	ID          string               `json:"id"`
	Kind        Kind                 `json:"kind"`
	Status      Status               `json:"status"`
	SessionID   string               `json:"sessionId,omitempty"` // CC session that spawned this job
	PID         int                  `json:"pid,omitempty"`
	CreatedAt   time.Time            `json:"createdAt"`
	StartedAt   time.Time            `json:"startedAt"`
	CompletedAt *time.Time           `json:"completedAt,omitempty"`
	DurationMs  int64                `json:"durationMs,omitempty"` // CompletedAt - StartedAt, in ms
	Request     Request              `json:"request"`
	Result      *schema.ReviewResult `json:"result,omitempty"`
	Error       string               `json:"error,omitempty"`
	LogPath     string               `json:"logPath"`
	Warnings    []string             `json:"warnings,omitempty"`
	Tokens      *TokenUsage          `json:"tokens,omitempty"`
}

// NewID returns a sortable, time-prefixed unique ID:
// "YYYYMMDDTHHmmss-XXXXXXXX" (8 hex chars random suffix).
func NewID() string {
	ts := time.Now().UTC().Format("20060102T150405")
	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	return fmt.Sprintf("%s-%s", ts, hex.EncodeToString(rnd[:]))
}

// Save serializes a job record to {workspace}/jobs/{id}.json atomically.
func Save(ws state.WorkspaceDir, j Job) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteAtomic(ws.JobPath(j.ID), data, 0o644)
}

func Load(ws state.WorkspaceDir, id string) (Job, error) {
	var j Job
	data, err := os.ReadFile(ws.JobPath(id))
	if err != nil {
		return j, err
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return j, err
	}
	return j, nil
}

// List returns jobs sorted newest first.
func List(ws state.WorkspaceDir) ([]Job, error) {
	ids, err := ws.ListJobIDs()
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(ids))
	for _, id := range ids {
		if j, err := Load(ws, id); err == nil {
			jobs = append(jobs, j)
		}
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}

// ListBySession returns jobs whose SessionID equals session, sorted newest-first.
// Empty session returns all jobs (equivalent to List).
func ListBySession(ws state.WorkspaceDir, session string) ([]Job, error) {
	all, err := List(ws)
	if err != nil {
		return nil, err
	}
	if session == "" {
		return all, nil
	}
	out := make([]Job, 0, len(all))
	for _, j := range all {
		if j.SessionID == session {
			out = append(out, j)
		}
	}
	return out, nil
}
