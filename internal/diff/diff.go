package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
)

type Bundle struct {
	TargetLabel string
	Diff        string
	Untracked   []UntrackedFile
	TotalBytes  int
	Truncated   []string
	Warnings    []string
}

type UntrackedFile struct {
	Path    string
	Content string
	Bytes   int
}

func (b Bundle) IsEmpty() bool {
	return strings.TrimSpace(b.Diff) == "" && len(b.Untracked) == 0
}

// CollectWorkingTree gathers a working-tree diff plus untracked text files,
// capping total payload size by dropping the largest contributors past the
// cap. v0.1 only supports working-tree target.
func CollectWorkingTree(cwd string) (Bundle, error) {
	bundle := Bundle{TargetLabel: "working tree"}

	diffStr, err := git.WorkingTreeDiff(cwd)
	if err != nil {
		return bundle, xerrors.Diff("collect_diff", fmt.Sprintf("git diff failed: %v", err), "")
	}
	bundle.Diff = diffStr
	bundle.TotalBytes = len(diffStr)

	untrackedPaths, err := git.UntrackedFiles(cwd)
	if err == nil {
		root, _ := git.RootOf(cwd)
		for _, rel := range untrackedPaths {
			abs := filepath.Join(root, rel)
			info, statErr := os.Stat(abs)
			if statErr != nil || info.IsDir() {
				continue
			}
			if info.Size() > 64*1024 {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("skipped untracked file >64KB: %s", rel))
				continue
			}
			data, readErr := os.ReadFile(abs)
			if readErr != nil {
				continue
			}
			if !isTextLike(data) {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("skipped binary untracked file: %s", rel))
				continue
			}
			bundle.Untracked = append(bundle.Untracked, UntrackedFile{
				Path:    rel,
				Content: string(data),
				Bytes:   len(data),
			})
			bundle.TotalBytes += len(data)
		}
	}

	if bundle.TotalBytes > config.MaxDiffBytes {
		bundle = applyCap(bundle)
	}

	return bundle, nil
}

func applyCap(b Bundle) Bundle {
	for len(b.Untracked) > 0 && b.TotalBytes > config.MaxDiffBytes {
		idx := largestUntrackedIdx(b.Untracked)
		dropped := b.Untracked[idx]
		b.Untracked = append(b.Untracked[:idx], b.Untracked[idx+1:]...)
		b.TotalBytes -= dropped.Bytes
		b.Truncated = append(b.Truncated, dropped.Path)
	}

	if b.TotalBytes > config.MaxDiffBytes {
		over := b.TotalBytes - config.MaxDiffBytes
		if over < len(b.Diff) {
			b.Diff = b.Diff[:len(b.Diff)-over] + "\n[... diff truncated ...]\n"
			b.TotalBytes = len(b.Diff)
			b.Truncated = append(b.Truncated, "(diff body trimmed)")
		}
	}

	b.Warnings = append(b.Warnings,
		fmt.Sprintf("diff exceeded %d byte cap; truncated %d items", config.MaxDiffBytes, len(b.Truncated)))
	return b
}

func largestUntrackedIdx(files []UntrackedFile) int {
	idx := 0
	for i, f := range files {
		if f.Bytes > files[idx].Bytes {
			idx = i
		}
	}
	return idx
}

func isTextLike(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	for i := 0; i < limit; i++ {
		b := data[i]
		if b == 0 {
			return false
		}
	}
	return true
}

func RenderForPrompt(b Bundle) string {
	var sb strings.Builder
	if strings.TrimSpace(b.Diff) != "" {
		sb.WriteString("### git diff (working tree)\n\n```diff\n")
		sb.WriteString(b.Diff)
		sb.WriteString("\n```\n\n")
	}
	for _, f := range b.Untracked {
		sb.WriteString(fmt.Sprintf("### untracked: %s\n\n```\n", f.Path))
		sb.WriteString(f.Content)
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}
