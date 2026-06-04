package index

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLocation_JSONRoundtrip(t *testing.T) {
	loc := Location{
		Name: "Foo",
		File: "internal/auth/auth.go",
		Line: 42,
		Kind: SymDef,
		Pkg:  "auth",
	}
	b, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Location
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(loc, got) {
		t.Fatalf("roundtrip: want %+v, got %+v", loc, got)
	}
}

func TestKind_StringRoundtrip(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{SymDef, "def"},
		{SymCall, "call"},
		{SymImport, "import"},
		{SymTypeRef, "type_ref"},
	}
	for _, c := range cases {
		if c.k.String() != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.k, c.k.String(), c.want)
		}
	}
}

func TestIndex_CurrentSchemaVersion(t *testing.T) {
	if CurrentSchemaVersion < 1 {
		t.Fatalf("CurrentSchemaVersion must be ≥1, got %d", CurrentSchemaVersion)
	}
}

func TestIndex_LookupDefs_Basic(t *testing.T) {
	idx := &Index{
		Version: CurrentSchemaVersion,
		Files: map[string]*FileIndex{
			"a.go": {
				Path: "a.go", Lang: "go",
				Defs: []Location{
					{Name: "Foo", File: "a.go", Line: 10, Kind: SymDef},
					{Name: "Bar", File: "a.go", Line: 20, Kind: SymDef},
				},
			},
			"b.go": {
				Path: "b.go", Lang: "go",
				Defs: []Location{
					{Name: "Foo", File: "b.go", Line: 5, Kind: SymDef, Pkg: "subpkg"},
				},
			},
		},
	}
	idx.RebuildLookups()

	got := idx.LookupDefs("Foo", "")
	if len(got) != 2 {
		t.Fatalf("LookupDefs Foo (no pkg): want 2, got %d", len(got))
	}

	gotPkg := idx.LookupDefs("Foo", "subpkg")
	if len(gotPkg) != 1 {
		t.Fatalf("LookupDefs Foo subpkg: want 1, got %d", len(gotPkg))
	}
	if gotPkg[0].File != "b.go" {
		t.Fatalf("LookupDefs Foo subpkg: want b.go, got %s", gotPkg[0].File)
	}

	none := idx.LookupDefs("Missing", "")
	if len(none) != 0 {
		t.Fatalf("LookupDefs Missing: want 0, got %d", len(none))
	}
}

func TestIndex_LookupRefs_ReturnsAllNonDefOccurrences(t *testing.T) {
	idx := &Index{
		Version: CurrentSchemaVersion,
		Files: map[string]*FileIndex{
			"caller.go": {
				Path: "caller.go", Lang: "go",
				Refs: []Location{
					{Name: "Foo", File: "caller.go", Line: 30, Kind: SymCall},
				},
			},
		},
	}
	idx.RebuildLookups()

	got := idx.LookupRefs("Foo", "")
	if len(got) != 1 {
		t.Fatalf("LookupRefs Foo: want 1, got %d", len(got))
	}
}
