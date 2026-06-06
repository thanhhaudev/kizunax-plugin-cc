// Package fanout contains the bucketing + worker-spawn + result-merge logic
// for kizunax binary-side fan-out reviews.
//
// Bucketing (this file) is pure logic — no I/O, no git, no exec. The dispatch
// layer collects the changed file list (via git or otherwise) and calls
// Bucket() to get the grouping.
package fanout

import (
	"sort"
	"strings"
)

// Bucket is one fan-out work unit: a path-prefix selector plus the files that
// would be reviewed under that selector.
type Bucket struct {
	Prefix string   // e.g. "api/cmd", "api", "misc", "."
	Files  []string // files this bucket covers (relative to repo root)
}

// Group groups changed files into review buckets following the v0.26.0 rules:
//
//   - Group by 1st path segment.
//   - If any bucket has >50 files, sub-group by 2nd segment.
//   - If total bucket count >10, merge smallest into "misc" until count ≤10.
//   - Drop empty buckets.
//
// Returns at least 1 bucket if input is non-empty. Returns empty slice when
// input is empty.
func Group(files []string) []Bucket {
	if len(files) == 0 {
		return []Bucket{}
	}

	// Step 1: normalize + group by 1st segment.
	byTop := map[string][]string{}
	for _, f := range files {
		f = strings.TrimPrefix(f, "./")
		f = strings.TrimSuffix(f, "/")
		if f == "" {
			continue
		}
		segs := strings.SplitN(f, "/", 2)
		top := segs[0]
		if len(segs) == 1 {
			// Root-level file. Bucket as ".".
			byTop["."] = append(byTop["."], f)
		} else {
			byTop[top] = append(byTop[top], f)
		}
	}

	// Step 2: sub-group any bucket with >50 files by 2nd segment.
	subBuckets := map[string][]string{}
	for top, fs := range byTop {
		if len(fs) <= 50 || top == "." {
			subBuckets[top] = fs
			continue
		}
		// Split by 2nd segment.
		by2nd := map[string][]string{}
		for _, f := range fs {
			parts := strings.SplitN(f, "/", 3)
			var prefix string
			if len(parts) >= 2 {
				prefix = parts[0] + "/" + parts[1]
			} else {
				prefix = parts[0]
			}
			by2nd[prefix] = append(by2nd[prefix], f)
		}
		for k, v := range by2nd {
			subBuckets[k] = v
		}
	}

	// Step 3: if >10 buckets, merge smallest into "misc" until count ≤10.
	if len(subBuckets) > 10 {
		type kv struct {
			prefix string
			files  []string
		}
		var sorted []kv
		for k, v := range subBuckets {
			sorted = append(sorted, kv{k, v})
		}
		// Sort ascending by file count (smallest first); break ties by prefix for stability.
		sort.Slice(sorted, func(i, j int) bool {
			if len(sorted[i].files) != len(sorted[j].files) {
				return len(sorted[i].files) < len(sorted[j].files)
			}
			return sorted[i].prefix < sorted[j].prefix
		})
		var misc []string
		// Keep merging the smallest bucket into misc until total ≤ 10.
		// We target 9 remaining named buckets + 1 misc = 10.
		for len(subBuckets) > 9 {
			smallest := sorted[0]
			sorted = sorted[1:]
			delete(subBuckets, smallest.prefix)
			misc = append(misc, smallest.files...)
		}
		if len(misc) > 0 {
			subBuckets["misc"] = misc
		}
	}

	// Step 4: drop empties + build []Bucket result.
	var result []Bucket
	for prefix, fs := range subBuckets {
		if len(fs) == 0 {
			continue
		}
		result = append(result, Bucket{Prefix: prefix, Files: fs})
	}
	return result
}
