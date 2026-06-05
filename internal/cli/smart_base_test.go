package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRun is a test helper that runs a git command in dir, failing the test
// with the captured output if the command exits non-zero.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
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

// initRepoOnBranch initializes a fresh git repo in dir on the given branch
// with one commit. Returns the directory for chaining.
func initRepoOnBranch(t *testing.T, dir, branch string) string {
	t.Helper()
	gitRun(t, dir, "init", "-b", branch)
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "seed")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func TestIsWorkingTreeDirty_CleanRepo(t *testing.T) {
	dir := initRepoOnBranch(t, t.TempDir(), "master")
	withCwd(t, dir)

	dirty, ok := isWorkingTreeDirty(dir)
	if !ok {
		t.Fatal("expected success on initialized repo")
	}
	if dirty {
		t.Errorf("expected clean working tree right after init+commit")
	}
}

func TestIsWorkingTreeDirty_UntrackedFile(t *testing.T) {
	dir := initRepoOnBranch(t, t.TempDir(), "master")
	withCwd(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "new"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirty, ok := isWorkingTreeDirty(dir)
	if !ok {
		t.Fatal("expected success")
	}
	if !dirty {
		t.Errorf("expected dirty with untracked file present")
	}
}

func TestIsWorkingTreeDirty_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if _, ok := isWorkingTreeDirty(dir); ok {
		t.Error("expected failure on non-git dir")
	}
}

func TestAutoDetectBaseRef_UpstreamWins(t *testing.T) {
	// Use a single repo as its own remote so we can set up tracking.
	dir := initRepoOnBranch(t, t.TempDir(), "develop")
	withCwd(t, dir)

	gitRun(t, dir, "checkout", "-b", "feature/x")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "f")
	gitRun(t, dir, "commit", "-m", "work")

	// Wire develop as the upstream of feature/x by faking a remote pointing
	// at this same repo and fetching.
	gitRun(t, dir, "remote", "add", "origin", dir)
	gitRun(t, dir, "fetch", "origin")
	gitRun(t, dir, "branch", "--set-upstream-to=origin/develop", "feature/x")

	got, err := autoDetectBaseRef()
	if err != nil {
		t.Fatalf("autoDetectBaseRef: %v", err)
	}
	if got != "origin/develop" {
		t.Errorf("expected 'origin/develop' (the upstream), got %q", got)
	}
}

func TestAutoDetectBaseRef_FallbackToDevelop(t *testing.T) {
	// Repo with develop branch but no upstream tracking on current branch.
	dir := initRepoOnBranch(t, t.TempDir(), "develop")
	withCwd(t, dir)

	gitRun(t, dir, "checkout", "-b", "feature/y")

	got, err := autoDetectBaseRef()
	if err != nil {
		t.Fatalf("autoDetectBaseRef: %v", err)
	}
	if got != "develop" {
		t.Errorf("expected fallback 'develop', got %q", got)
	}
}

func TestAutoDetectBaseRef_FallbackToMaster(t *testing.T) {
	dir := initRepoOnBranch(t, t.TempDir(), "master")
	withCwd(t, dir)

	gitRun(t, dir, "checkout", "-b", "feature/z")

	got, err := autoDetectBaseRef()
	if err != nil {
		t.Fatalf("autoDetectBaseRef: %v", err)
	}
	if got != "master" {
		t.Errorf("expected fallback 'master', got %q", got)
	}
}

// TestAutoDetectBaseRef_SkipsSelfUpstream is the regression test for the
// v0.22.1 bug: branch `feature/compare_order_phase2` tracks
// `origin/feature/compare_order_phase2` and the v0.20-v0.22.0 binary
// picked that ref, producing a 0-diff "review" of a single local file.
// After the fix, autoDetect should ignore self-upstream and fall back
// to master.
func TestAutoDetectBaseRef_SkipsSelfUpstream(t *testing.T) {
	dir := initRepoOnBranch(t, t.TempDir(), "master")
	withCwd(t, dir)

	gitRun(t, dir, "checkout", "-b", "feature/x")
	// Wire origin to point at this same repo and fetch so origin/feature/x exists.
	gitRun(t, dir, "remote", "add", "origin", dir)
	gitRun(t, dir, "fetch", "origin")
	gitRun(t, dir, "branch", "--set-upstream-to=origin/feature/x", "feature/x")

	got, err := autoDetectBaseRef()
	if err != nil {
		t.Fatalf("autoDetectBaseRef: %v", err)
	}
	if got != "master" {
		t.Errorf("expected fallback 'master' (self-upstream ignored), got %q", got)
	}
}

func TestIsSelfUpstream(t *testing.T) {
	cases := []struct {
		upstream, currentBranch string
		want                    bool
	}{
		{"origin/feature/x", "feature/x", true},
		{"origin/develop", "feature/x", false},
		{"origin/master", "master", true},
		{"upstream/main", "main", true},
		{"feature/x", "feature/x", true},
		{"", "feature/x", false},
		{"origin/feature/x", "", false},
		{"origin/feature/x-suffix", "feature/x", false},
	}
	for _, c := range cases {
		got := isSelfUpstream(c.upstream, c.currentBranch)
		if got != c.want {
			t.Errorf("isSelfUpstream(%q, %q) = %v, want %v", c.upstream, c.currentBranch, got, c.want)
		}
	}
}

func TestSuggestSmallerBaseRefs_NoSuggestionWhenSmall(t *testing.T) {
	dir := initRepoOnBranch(t, t.TempDir(), "master")
	withCwd(t, dir)
	// currentFileCount < 100 → no work, no suggestion.
	if _, _, ok := suggestSmallerBaseRefs("master", 50); ok {
		t.Errorf("expected no suggestion for small diff")
	}
}

func TestSuggestSmallerBaseRefs_SuggestsSmaller(t *testing.T) {
	// Construct a layered repo: master has 1 commit, develop has many commits
	// on top of master, feature has just one commit on top of develop. The
	// "diff vs master" file count is much larger than "diff vs develop".
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "master")
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "seed")
	gitRun(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "init")

	gitRun(t, dir, "checkout", "-b", "develop")
	for i := 0; i < 150; i++ {
		name := filepath.Join(dir, "dev_file_"+itoa(i))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "develop work")

	gitRun(t, dir, "checkout", "-b", "feature/small")
	if err := os.WriteFile(filepath.Join(dir, "feature_only.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "feature_only.txt")
	gitRun(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "feature commit")

	withCwd(t, dir)

	// Diff vs master ≈ 151 files; vs develop ≈ 1 file. Should suggest develop.
	suggested, count, ok := suggestSmallerBaseRefs("master", 151)
	if !ok {
		t.Fatal("expected a suggestion when diff vs develop is much smaller than vs master")
	}
	if suggested != "develop" {
		t.Errorf("expected 'develop' suggestion, got %q", suggested)
	}
	if count > 10 {
		t.Errorf("expected suggestion count near 1, got %d", count)
	}
}

// itoa is a tiny dependency-free int→string for test fixture filenames.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
