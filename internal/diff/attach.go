package diff

import (
	"fmt"
	"sort"
)

// ReferenceInput is the resolver-output shape consumed by AttachReferenced.
// We keep this separate from resolver.Reference to avoid an import cycle
// (resolver imports diff for Bundle definitions).
type ReferenceInput struct {
	Path    string
	Excerpt string
	Symbols []string
}

// AttachReferenced merges enrichment references into the bundle,
// respecting the budget cap (256 KiB by default). Mutates b in place.
// Warnings about dropped/oversized references are appended to b.Warnings.
func AttachReferenced(b *Bundle, refs []ReferenceInput, budgetBytes int) {
	if len(refs) == 0 || budgetBytes <= 0 {
		return
	}

	// Dedup by Path: if same file appears twice (different symbols),
	// merge the symbol lists.
	merged := map[string]*ReferenceInput{}
	order := []string{} // preserve first-seen order
	for i := range refs {
		r := refs[i]
		if existing, ok := merged[r.Path]; ok {
			existing.Symbols = appendUnique(existing.Symbols, r.Symbols...)
			continue
		}
		cp := r
		merged[r.Path] = &cp
		order = append(order, r.Path)
	}

	// Priority sort: (-len(symbols), len(excerpt), path) — deterministic.
	sortable := make([]*ReferenceInput, 0, len(order))
	for _, p := range order {
		sortable = append(sortable, merged[p])
	}
	sort.SliceStable(sortable, func(i, j int) bool {
		if len(sortable[i].Symbols) != len(sortable[j].Symbols) {
			return len(sortable[i].Symbols) > len(sortable[j].Symbols)
		}
		if len(sortable[i].Excerpt) != len(sortable[j].Excerpt) {
			return len(sortable[i].Excerpt) < len(sortable[j].Excerpt)
		}
		return sortable[i].Path < sortable[j].Path
	})

	// Greedy fit.
	var kept []ReferencedFile
	used := 0
	dropped := 0
	for _, r := range sortable {
		// Account for path/header overhead in the rendered template.
		// Rough estimate: path + 2 fence markers + symbol list = ~80 bytes.
		cost := len(r.Excerpt) + 80
		if used+cost > budgetBytes {
			dropped++
			continue
		}
		used += cost
		kept = append(kept, ReferencedFile{
			Path:    r.Path,
			Excerpt: r.Excerpt,
			Symbols: r.Symbols,
		})
	}

	b.ReferencedFiles = kept
	if dropped > 0 {
		b.Warnings = append(b.Warnings,
			fmt.Sprintf("referenced files dropped: %d of %d (kept %d) due to %d-byte cap",
				dropped, len(sortable), len(kept), budgetBytes))
	}
}

func appendUnique(have []string, add ...string) []string {
	seen := map[string]bool{}
	for _, s := range have {
		seen[s] = true
	}
	for _, s := range add {
		if !seen[s] {
			have = append(have, s)
			seen[s] = true
		}
	}
	return have
}
