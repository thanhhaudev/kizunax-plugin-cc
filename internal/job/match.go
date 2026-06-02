package job

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrJobNotFound    = errors.New("job not found")
	ErrAmbiguousJobID = errors.New("ambiguous job id prefix")
)

// MatchByPrefix returns the job whose ID equals or uniquely starts with ref.
// Empty ref returns jobs[0] (caller should pre-sort newest-first).
// Ambiguous prefix returns ErrAmbiguousJobID; no match returns ErrJobNotFound.
func MatchByPrefix(jobs []Job, ref string) (Job, error) {
	if ref == "" {
		if len(jobs) == 0 {
			return Job{}, ErrJobNotFound
		}
		return jobs[0], nil
	}
	for _, j := range jobs {
		if j.ID == ref {
			return j, nil
		}
	}
	var matches []Job
	for _, j := range jobs {
		if strings.HasPrefix(j.ID, ref) {
			matches = append(matches, j)
		}
	}
	switch len(matches) {
	case 0:
		return Job{}, fmt.Errorf("%w: %q", ErrJobNotFound, ref)
	case 1:
		return matches[0], nil
	default:
		return Job{}, fmt.Errorf("%w: %q (matches: %d). Use a longer id.", ErrAmbiguousJobID, ref, len(matches))
	}
}
