package index

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLocation_JSONRoundtrip(t *testing.T) {
	loc := Location{
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
