package git

import (
	"os/exec"
	"strings"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

func EnsureRepo(cwd string) error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.User(
			"not_a_git_repo",
			"not inside a git repository",
			"run from a git working tree, or 'git init' first",
		)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return xerrors.User("not_a_git_repo", "not inside a git work tree", "")
	}
	return nil
}

func RootOf(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", xerrors.User("git_root", "cannot determine git root", "")
	}
	return strings.TrimSpace(string(out)), nil
}

func Status(cwd string) (string, error) {
	cmd := exec.Command("git", "status", "--short", "--untracked-files=all")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func WorkingTreeDiff(cwd string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		var staged []byte
		c2 := exec.Command("git", "diff")
		c2.Dir = cwd
		staged, _ = c2.Output()
		return string(staged), nil
	}
	return string(out), nil
}

func UntrackedFiles(cwd string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}
