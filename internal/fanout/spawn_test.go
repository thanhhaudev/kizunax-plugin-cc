//go:build !windows

package fanout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubBinary writes a shell script to tempdir that echoes its args + exits 0.
// Returns the script path. Skip test on Windows (the shell script approach
// doesn't apply).
func stubBinary(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "kizunax-stub")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSpawn_NoBinaryPath_Errors(t *testing.T) {
	_, err := Run(context.Background(), []Bucket{{Prefix: "api"}}, SpawnOptions{
		Subcommand: "review",
	})
	if err == nil {
		t.Error("expected error for empty BinaryPath")
	}
}

func TestSpawn_NoSubcommand_Errors(t *testing.T) {
	_, err := Run(context.Background(), []Bucket{{Prefix: "api"}}, SpawnOptions{
		BinaryPath: "/usr/bin/true",
	})
	if err == nil {
		t.Error("expected error for empty Subcommand")
	}
}

func TestSpawn_EmptyBuckets_ReturnsEmpty(t *testing.T) {
	got, err := Run(context.Background(), nil, SpawnOptions{
		BinaryPath: "/usr/bin/true",
		Subcommand: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d results, want 0", len(got))
	}
}

func TestSpawn_HappyPath_OrderPreserved(t *testing.T) {
	// Stub echoes "BUCKET=$paths_arg" to stdout.
	stub := stubBinary(t, `
for arg in "$@"; do
  case "$prev" in
    --paths) echo "BUCKET=$arg" ;;
  esac
  prev="$arg"
done
`)
	buckets := []Bucket{
		{Prefix: "api/cmd", Files: []string{"api/cmd/x.go"}},
		{Prefix: "web", Files: []string{"web/y.tsx"}},
		{Prefix: "db", Files: []string{"db/z.sql"}},
	}
	got, err := Run(context.Background(), buckets, SpawnOptions{
		BinaryPath:  stub,
		Subcommand:  "review",
		Concurrency: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("result count: got %d, want 3", len(got))
	}
	// Order matches input.
	for i, b := range buckets {
		if got[i].Bucket.Prefix != b.Prefix {
			t.Errorf("results[%d].Bucket.Prefix: got %q, want %q", i, got[i].Bucket.Prefix, b.Prefix)
		}
		if !strings.Contains(got[i].Stdout, "BUCKET="+b.Prefix) {
			t.Errorf("results[%d] stdout missing BUCKET marker: %q", i, got[i].Stdout)
		}
		if got[i].ExitCode != 0 {
			t.Errorf("results[%d] exit code: got %d, want 0", i, got[i].ExitCode)
		}
		if got[i].Err != nil {
			t.Errorf("results[%d] err: %v", i, got[i].Err)
		}
	}
}

func TestSpawn_PerBucketTimeout(t *testing.T) {
	// Stub sleeps 5 seconds; timeout is 100ms.
	stub := stubBinary(t, "sleep 5")
	got, err := Run(context.Background(), []Bucket{{Prefix: "api", Files: []string{"api/x.go"}}}, SpawnOptions{
		BinaryPath:       stub,
		Subcommand:       "review",
		PerBucketTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Err == nil {
		t.Error("expected per-bucket timeout error")
	}
}

func TestSpawn_RootAndMiscPrefixesSkipPathsFlag(t *testing.T) {
	// Stub records argv so we can assert --paths is absent for "." and "misc".
	stub := stubBinary(t, `echo "ARGS:$@"`)
	buckets := []Bucket{
		{Prefix: ".", Files: []string{"README.md"}},
		{Prefix: "misc", Files: []string{"x/y.go"}},
		{Prefix: "api", Files: []string{"api/main.go"}},
	}
	got, err := Run(context.Background(), buckets, SpawnOptions{
		BinaryPath: stub,
		Subcommand: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got[0].Stdout, "--paths") {
		t.Errorf("root bucket should not have --paths; stdout=%q", got[0].Stdout)
	}
	if strings.Contains(got[1].Stdout, "--paths") {
		t.Errorf("misc bucket should not have --paths; stdout=%q", got[1].Stdout)
	}
	if !strings.Contains(got[2].Stdout, "--paths api") {
		t.Errorf("api bucket should have --paths api; stdout=%q", got[2].Stdout)
	}
}

func TestSpawn_NonZeroExit_CapturedNotPanicking(t *testing.T) {
	stub := stubBinary(t, "exit 3")
	got, err := Run(context.Background(), []Bucket{{Prefix: "api", Files: []string{"api/x.go"}}}, SpawnOptions{
		BinaryPath: stub,
		Subcommand: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ExitCode != 3 {
		t.Errorf("exit code: got %d, want 3", got[0].ExitCode)
	}
	if got[0].Err == nil {
		t.Error("expected Err on non-zero exit")
	}
}

func TestSpawn_ContextCancel_KillsRunning(t *testing.T) {
	// Spawn 4 long-running workers, cancel after 50ms.
	stub := stubBinary(t, "sleep 5")
	buckets := []Bucket{
		{Prefix: "a", Files: []string{"a/x"}},
		{Prefix: "b", Files: []string{"b/x"}},
		{Prefix: "c", Files: []string{"c/x"}},
		{Prefix: "d", Files: []string{"d/x"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	got, err := Run(ctx, buckets, SpawnOptions{
		BinaryPath:  stub,
		Subcommand:  "review",
		Concurrency: 4,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("cancel ignored: elapsed %v should be < 2s", elapsed)
	}
	// Each worker should have non-nil Err (killed/cancelled).
	for i, r := range got {
		if r.Err == nil {
			t.Errorf("results[%d]: expected err on cancel, got nil", i)
		}
	}
}

// suppress vet warning
var _ = strings.Contains
var _ = exec.Cmd{}
