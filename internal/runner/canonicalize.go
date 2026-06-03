package runner

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/schema"
)

// canonicalizeFindings rewrites finding.File to the full repo-relative path
// when the LLM emitted only a basename and exactly one path in `paths`
// matches that basename. Mutates findings in place.
//
// Behaviour:
//   - finding.File exactly matches a known path → keep as-is.
//   - finding.File is a basename matching exactly one known path → rewrite.
//   - basename matches multiple paths → keep + return a warning naming the
//     candidates (user must disambiguate manually).
//   - no match at all → keep silently (LLM may have hallucinated; a separate
//     concern out of scope for this function).
//   - empty finding.File or empty paths → no-op.
//
// Warnings are returned as strings so the caller (runner.Run) can decide
// how to surface them (stderr today; could be embedded in render later).
func canonicalizeFindings(findings []schema.Finding, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	// Build the exact-path set and basename → []full-paths index.
	exact := make(map[string]struct{}, len(paths))
	byBase := make(map[string][]string)
	for _, p := range paths {
		if p == "" {
			continue
		}
		exact[p] = struct{}{}
		base := path.Base(p)
		byBase[base] = append(byBase[base], p)
	}

	var warnings []string
	for i := range findings {
		f := findings[i].File
		if f == "" {
			continue
		}
		if _, ok := exact[f]; ok {
			continue
		}
		// f is not an exact known path. Try basename canonicalization.
		base := path.Base(f)
		candidates := byBase[base]
		switch len(candidates) {
		case 0:
			// No match. LLM possibly hallucinated. Leave alone.
		case 1:
			findings[i].File = candidates[0]
		default:
			// Ambiguous. Keep basename + warn.
			sorted := append([]string{}, candidates...)
			sort.Strings(sorted)
			warnings = append(warnings, fmt.Sprintf(
				"finding %q references ambiguous file %q — candidates: %s",
				findings[i].Title, f, strings.Join(sorted, ", "),
			))
		}
	}
	return warnings
}
