package fanout

import (
	"sort"
	"testing"
)

func TestBucket_Empty(t *testing.T) {
	got := Group(nil)
	if len(got) != 0 {
		t.Errorf("empty input: got %d buckets, want 0", len(got))
	}
	got = Group([]string{})
	if len(got) != 0 {
		t.Errorf("empty slice: got %d buckets, want 0", len(got))
	}
}

func TestBucket_SingleTopLevelDir(t *testing.T) {
	files := []string{"api/main.go", "api/server.go", "api/db.go"}
	got := Group(files)
	if len(got) != 1 {
		t.Fatalf("got %d buckets, want 1: %+v", len(got), got)
	}
	if got[0].Prefix != "api" {
		t.Errorf("prefix: got %q, want %q", got[0].Prefix, "api")
	}
	if len(got[0].Files) != 3 {
		t.Errorf("file count: got %d, want 3", len(got[0].Files))
	}
}

func TestBucket_MultipleTopLevelDirs(t *testing.T) {
	files := []string{
		"api/main.go", "api/server.go",
		"web/app.tsx", "web/index.html",
		"db/schema.sql",
	}
	got := Group(files)
	if len(got) != 3 {
		t.Fatalf("got %d buckets, want 3: %+v", len(got), got)
	}
	p := make([]string, len(got))
	for i, b := range got {
		p[i] = b.Prefix
	}
	sort.Strings(p)
	want := []string{"api", "db", "web"}
	for i, w := range want {
		if p[i] != w {
			t.Errorf("prefix[%d]: got %q, want %q", i, p[i], w)
		}
	}
}

func TestBucket_RootLevelFiles(t *testing.T) {
	files := []string{"README.md", "go.mod", "Makefile"}
	got := Group(files)
	if len(got) != 1 {
		t.Fatalf("got %d buckets, want 1: %+v", len(got), got)
	}
	if got[0].Prefix != "." {
		t.Errorf("root prefix: got %q, want %q", got[0].Prefix, ".")
	}
}

func TestBucket_LargeDir_SubGrouped(t *testing.T) {
	// 60 files under api/, half in cmd/ half in db/ — should sub-group.
	var files []string
	for i := 0; i < 30; i++ {
		files = append(files, fmtPath("api/cmd", i))
	}
	for i := 0; i < 30; i++ {
		files = append(files, fmtPath("api/db", i))
	}
	got := Group(files)
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2 (sub-grouped api/cmd + api/db): %+v", len(got), prefixes(got))
	}
	wantPrefixes := map[string]bool{"api/cmd": false, "api/db": false}
	for _, b := range got {
		if _, ok := wantPrefixes[b.Prefix]; ok {
			wantPrefixes[b.Prefix] = true
		}
	}
	for p, seen := range wantPrefixes {
		if !seen {
			t.Errorf("missing sub-bucket %q; got prefixes: %v", p, prefixes(got))
		}
	}
}

func TestBucket_TooManyTopLevel_MergedIntoMisc(t *testing.T) {
	// 15 top-level dirs each with 1 file → should merge smallest 6 into misc, leaving 9 + 1 = 10.
	// Adjust expectation: rule is "merge smallest into misc until count ≤ 10".
	var files []string
	dirs := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o"}
	for _, d := range dirs {
		files = append(files, d+"/main.go")
	}
	got := Group(files)
	if len(got) > 10 {
		t.Errorf("got %d buckets, want ≤10", len(got))
	}
	var sawMisc bool
	for _, b := range got {
		if b.Prefix == "misc" {
			sawMisc = true
			if len(b.Files) < 2 {
				t.Errorf("misc bucket should have ≥2 files; got %d: %+v", len(b.Files), b.Files)
			}
		}
	}
	if !sawMisc {
		t.Errorf("expected a misc bucket when total >10: %+v", prefixes(got))
	}
}

func TestBucket_LeadingDotSlash_Stripped(t *testing.T) {
	files := []string{"./api/main.go", "./api/server.go"}
	got := Group(files)
	if len(got) != 1 || got[0].Prefix != "api" {
		t.Errorf("got %+v, want single bucket prefix=api", prefixes(got))
	}
}

func fmtPath(prefix string, i int) string {
	return prefix + "/file" + itoa(i) + ".go"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func prefixes(bs []Bucket) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Prefix
	}
	return out
}
