package job

import (
	"errors"
	"strings"
	"testing"
)

func TestMatchByPrefix_ExactID(t *testing.T) {
	jobs := []Job{{ID: "20260602T100000-aaaa"}, {ID: "20260602T100001-bbbb"}}
	got, err := MatchByPrefix(jobs, "20260602T100000-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "20260602T100000-aaaa" {
		t.Errorf("got %s", got.ID)
	}
}

func TestMatchByPrefix_UniquePrefix(t *testing.T) {
	jobs := []Job{{ID: "20260602T100000-aaaa"}, {ID: "20260602T100001-bbbb"}}
	got, err := MatchByPrefix(jobs, "20260602T100000")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "20260602T100000-aaaa" {
		t.Errorf("got %s", got.ID)
	}
}

func TestMatchByPrefix_AmbiguousPrefix(t *testing.T) {
	jobs := []Job{{ID: "20260602T100000-aaaa"}, {ID: "20260602T100000-bbbb"}}
	_, err := MatchByPrefix(jobs, "20260602T100000")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrAmbiguousJobID) {
		t.Errorf("expected ErrAmbiguousJobID, got %v", err)
	}
	if !strings.Contains(err.Error(), "20260602T100000") {
		t.Errorf("error should include the prefix: %v", err)
	}
}

func TestMatchByPrefix_NotFound(t *testing.T) {
	jobs := []Job{{ID: "AAAA"}}
	_, err := MatchByPrefix(jobs, "ZZZZ")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestMatchByPrefix_EmptyRef_ReturnsFirst(t *testing.T) {
	// First in list (caller should pre-sort newest-first).
	jobs := []Job{{ID: "NEW"}, {ID: "OLD"}}
	got, err := MatchByPrefix(jobs, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "NEW" {
		t.Errorf("got %s, want NEW", got.ID)
	}
}

func TestMatchByPrefix_EmptyRef_EmptyList_NotFound(t *testing.T) {
	_, err := MatchByPrefix(nil, "")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound on empty list, got %v", err)
	}
}
