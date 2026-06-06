//go:build !windows

package fanout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const (
	defaultConcurrency      = 4
	defaultPerBucketTimeout = 15 * time.Minute
)

// SpawnOptions controls how workers are launched.
type SpawnOptions struct {
	// BinaryPath is the absolute path of the kizunax binary to re-invoke as
	// workers. The caller should resolve this via os.Executable() before
	// passing it in (we don't call os.Executable() here to keep this package
	// pure / testable).
	BinaryPath string

	// Subcommand is "review" or "adversarial-review".
	Subcommand string

	// BaseArgs are the user's original flags that should be forwarded to
	// every worker (e.g., --base master, --provider anthropic, --quiet,
	// --no-expand, --model X). The dispatch layer (F4) curates this — F2
	// just passes it through. Workers ALWAYS get --quiet + --no-expand
	// added on top (handled here).
	BaseArgs []string

	// Concurrency is the max worker count per batch. Default 4 if zero.
	Concurrency int

	// PerBucketTimeout caps each worker. Default 15 minutes if zero.
	PerBucketTimeout time.Duration

	// WorkingDir is the cwd to set for each worker (usually the repo root).
	// If empty, workers inherit parent cwd.
	WorkingDir string

	// ProgressFn is called once per completed bucket. Optional — pass nil
	// to disable progress reporting. Signature: (bucketIndex, total, prefix).
	ProgressFn func(done, total int, prefix string)
}

// BucketResult is the outcome of one bucket worker.
type BucketResult struct {
	Bucket   Bucket
	Stdout   string // raw rendered markdown from the worker, empty if Err != nil
	Stderr   string // captured stderr (warnings / [verbose] lines)
	ExitCode int
	Duration time.Duration
	Err      error // non-nil for timeout, spawn failure, or non-zero exit
}

// Run spawns workers for each bucket and collects results. The caller's ctx
// can cancel the whole fan-out (e.g., on SIGINT) — pending workers are killed
// via process-group signal.
//
// Returns the results in INPUT ORDER (matches buckets[i] ↔ results[i]). On
// any per-bucket failure, that BucketResult has Err set but Run does not
// abort other buckets — the caller decides what to do with partial results.
//
// Run itself only errors when SpawnOptions is invalid (empty BinaryPath /
// Subcommand). All other failures are surfaced per-bucket.
func Run(ctx context.Context, buckets []Bucket, opts SpawnOptions) ([]BucketResult, error) {
	if opts.BinaryPath == "" {
		return nil, errors.New("fanout: BinaryPath must not be empty")
	}
	if opts.Subcommand == "" {
		return nil, errors.New("fanout: Subcommand must not be empty")
	}

	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultConcurrency
	}
	if opts.PerBucketTimeout <= 0 {
		opts.PerBucketTimeout = defaultPerBucketTimeout
	}

	if len(buckets) == 0 {
		return []BucketResult{}, nil
	}

	results := make([]BucketResult, len(buckets))
	total := len(buckets)
	done := 0

	// Process in batches of Concurrency to enforce heat bound.
	for batchStart := 0; batchStart < total; batchStart += opts.Concurrency {
		batchEnd := batchStart + opts.Concurrency
		if batchEnd > total {
			batchEnd = total
		}
		batch := buckets[batchStart:batchEnd]

		type indexedResult struct {
			idx int
			res BucketResult
		}
		ch := make(chan indexedResult, len(batch))

		for i, bucket := range batch {
			globalIdx := batchStart + i
			go func(idx int, b Bucket) {
				res := runWorker(ctx, b, opts)
				ch <- indexedResult{idx: idx, res: res}
			}(globalIdx, bucket)
		}

		// Collect all results from this batch before starting the next.
		for range batch {
			ir := <-ch
			results[ir.idx] = ir.res
			done++
			if opts.ProgressFn != nil {
				opts.ProgressFn(done, total, results[ir.idx].Bucket.Prefix)
			}
		}
	}

	return results, nil
}

// runWorker executes one bucket subprocess and returns its result.
func runWorker(ctx context.Context, bucket Bucket, opts SpawnOptions) BucketResult {
	start := time.Now()

	args := buildArgs(bucket, opts)

	// Create a context that respects both the parent cancellation and the
	// per-bucket timeout.
	workerCtx, cancel := context.WithTimeout(ctx, opts.PerBucketTimeout)
	defer cancel()

	cmd := exec.CommandContext(workerCtx, opts.BinaryPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // new process group so kill -pgid works cleanly
	}
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	dur := time.Since(start)

	res := BucketResult{
		Bucket:   bucket,
		Stderr:   stderrBuf.String(),
		Duration: dur,
	}

	if err != nil {
		// Check if ctx was cancelled / timed out.
		if workerCtx.Err() != nil {
			res.Err = fmt.Errorf("bucket %q: %w", bucket.Prefix, workerCtx.Err())
		} else {
			res.Err = fmt.Errorf("bucket %q: %w", bucket.Prefix, err)
		}

		// Still capture exit code if available.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		}
		return res
	}

	res.Stdout = stdoutBuf.String()
	res.ExitCode = 0
	return res
}

// buildArgs constructs the argument list for a worker subprocess.
func buildArgs(bucket Bucket, opts SpawnOptions) []string {
	// Strip any existing --paths, --quiet, --no-expand from BaseArgs to avoid
	// duplication.
	base := filterArgs(opts.BaseArgs, "--paths", "--quiet", "--no-expand")

	args := make([]string, 0, len(base)+5)
	args = append(args, opts.Subcommand)
	args = append(args, base...)

	// Add --paths for non-root, non-misc buckets.
	if bucket.Prefix != "." && bucket.Prefix != "misc" {
		args = append(args, "--paths", bucket.Prefix)
	}

	args = append(args, "--quiet", "--no-expand")
	return args
}

// filterArgs returns a copy of args with any occurrence of the named flags
// (and their following value argument) removed. Handles both "--flag value"
// (two-token) and boolean flags "--flag" (single-token) forms. For the flags
// we strip here (--paths takes a value; --quiet and --no-expand are boolean),
// we use the flag name list to decide.
func filterArgs(args []string, flags ...string) []string {
	takesValue := map[string]bool{
		"--paths": true,
	}
	isFlag := make(map[string]bool, len(flags))
	for _, f := range flags {
		isFlag[f] = true
	}

	out := make([]string, 0, len(args))
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if isFlag[a] {
			if takesValue[a] {
				skip = true // skip next token (the value)
			}
			continue
		}
		out = append(out, a)
	}
	return out
}
