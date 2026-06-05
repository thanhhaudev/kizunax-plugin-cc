package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initBareTestRepo creates a git repo in dir with one commit on branch <branch>.
// If origin is non-empty, also sets remote origin pointing to itself and sets
// origin/HEAD to <origin>.
func initBareTestRepo(t *testing.T, dir, branch, origin string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", branch)
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "f")
	run("commit", "-m", "init")
	if origin != "" {
		// Point a fake "origin" at this same repo, then mirror-fetch so
		// refs/remotes/origin/<origin> exists, then set origin/HEAD.
		run("remote", "add", "origin", dir)
		run("fetch", "origin")
		run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+origin)
	}
}

func withCwd(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func TestResolveBaseRef_Empty(t *testing.T) {
	got, sub, err := resolveBaseRef("")
	if err != nil || sub || got != "" {
		t.Fatalf("expected ('', false, nil); got (%q, %v, %v)", got, sub, err)
	}
}

func TestResolveBaseRef_Exists(t *testing.T) {
	dir := t.TempDir()
	initBareTestRepo(t, dir, "master", "")
	withCwd(t, dir)

	got, sub, err := resolveBaseRef("master")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub {
		t.Errorf("expected substituted=false")
	}
	if got != "master" {
		t.Errorf("expected 'master', got %q", got)
	}
}

func TestResolveBaseRef_FallbackToDefault(t *testing.T) {
	dir := t.TempDir()
	initBareTestRepo(t, dir, "master", "master")
	withCwd(t, dir)

	got, sub, err := resolveBaseRef("main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sub {
		t.Errorf("expected substituted=true")
	}
	if got != "master" {
		t.Errorf("expected fallback 'master', got %q", got)
	}
}

func TestResolveBaseRef_NoRemote(t *testing.T) {
	dir := t.TempDir()
	initBareTestRepo(t, dir, "master", "")
	withCwd(t, dir)

	_, _, err := resolveBaseRef("nonexistent-branch")
	if err == nil {
		t.Fatal("expected error when ref missing and no remote")
	}
}
