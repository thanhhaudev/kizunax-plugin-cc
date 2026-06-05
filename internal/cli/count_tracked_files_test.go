package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCountTrackedFiles_RealRepo(t *testing.T) {
	// Use this repo itself — it has a stable set of tracked files.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Walk up to repo root (look for go.mod).
	dir := cwd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		dir = filepath.Dir(dir)
	}

	got, ok := countTrackedFilesCheaply(dir)
	if !ok {
		t.Fatal("expected success on a real git repo")
	}
	if got < 50 {
		t.Errorf("expected at least 50 tracked files in this repo, got %d", got)
	}
}

func TestCountTrackedFiles_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if _, ok := countTrackedFilesCheaply(dir); ok {
		t.Error("expected failure on a non-git dir")
	}
}

func TestCountTrackedFiles_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "master")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	got, ok := countTrackedFilesCheaply(dir)
	if !ok {
		t.Fatal("expected success on initialized git repo")
	}
	if got != 0 {
		t.Errorf("expected 0 files in empty repo, got %d", got)
	}
}
