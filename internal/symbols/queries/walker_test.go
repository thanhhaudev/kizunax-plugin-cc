//go:build !lite

package queries

import (
	"context"
	"os"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols/treesitter"
)

func TestCaptureToSymbol_Mapping(t *testing.T) {
	src := []byte("<?php\nfunction login() {}\n")
	cases := []struct {
		captureName string
		startByte   uint32
		endByte     uint32
		wantKind    symbols.SymbolKind
		wantName    string
	}{
		{"name.definition.function", 15, 20, symbols.SymDef, "login"},
		{"name.reference.call", 15, 20, symbols.SymCall, "login"},
		{"name.reference.import", 15, 20, symbols.SymImport, "login"},
		{"name.reference.type", 15, 20, symbols.SymTypeRef, "login"},
	}
	for _, c := range cases {
		cap := treesitter.Capture{
			Name: c.captureName, StartByte: c.startByte, EndByte: c.endByte,
		}
		sym, ok := CaptureToSymbol(cap, src, "auth.php", 1)
		if !ok {
			t.Errorf("CaptureToSymbol(%q) returned ok=false", c.captureName)
			continue
		}
		if sym.Kind != c.wantKind || sym.Name != c.wantName {
			t.Errorf("CaptureToSymbol(%q): got Kind=%v Name=%q, want Kind=%v Name=%q",
				c.captureName, sym.Kind, sym.Name, c.wantKind, c.wantName)
		}
	}
}

func TestCaptureToSymbol_UnrecognizedName_ReturnsFalse(t *testing.T) {
	cap := treesitter.Capture{Name: "name.something.else", StartByte: 0, EndByte: 4}
	if _, ok := CaptureToSymbol(cap, []byte("test"), "x", 1); ok {
		t.Fatal("expected ok=false for unrecognized capture name")
	}
}

func TestCaptureToSymbol_OutOfBoundsByteRange_ReturnsFalse(t *testing.T) {
	src := []byte("hi")
	cap := treesitter.Capture{Name: "name.definition.function", StartByte: 0, EndByte: 100}
	if _, ok := CaptureToSymbol(cap, src, "x", 1); ok {
		t.Fatal("expected ok=false for out-of-bounds end")
	}
	cap2 := treesitter.Capture{Name: "name.definition.function", StartByte: 5, EndByte: 3}
	if _, ok := CaptureToSymbol(cap2, src, "x", 1); ok {
		t.Fatal("expected ok=false for start >= end")
	}
}

func TestScanCaptures_ComputesLines(t *testing.T) {
	src := []byte("line1\nline2\nfunc foo() {}\n")
	caps := []treesitter.Capture{
		// "foo" starts after "line1\nline2\nfunc " = 17 bytes, ends at 20
		{Name: "name.definition.function", StartByte: 17, EndByte: 20},
	}
	syms := ScanCaptures(caps, src, "x.go")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol, got %d: %+v", len(syms), syms)
	}
	if syms[0].Name != "foo" {
		t.Errorf("expected name=foo, got %q", syms[0].Name)
	}
	if syms[0].Line != 3 {
		t.Errorf("expected line=3, got %d", syms[0].Line)
	}
	if syms[0].File != "x.go" {
		t.Errorf("expected file=x.go, got %q", syms[0].File)
	}
}

func TestScanCaptures_DropsUnknown(t *testing.T) {
	caps := []treesitter.Capture{
		{Name: "name.definition.function", StartByte: 0, EndByte: 3},
		{Name: "unknown.capture", StartByte: 0, EndByte: 3},
	}
	syms := ScanCaptures(caps, []byte("foo"), "x")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol (unknown dropped), got %d", len(syms))
	}
}

func loadPhpGrammar(t *testing.T) []byte {
	t.Helper()
	path := "../../../test-fixtures/tree-sitter-php.wasm"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("php grammar fixture not present: %v", err)
	}
	return data
}

// TestPhpQuery_ExtractsKnownSymbols verifies the full LoadGrammar →
// NewQuery → Parse → Exec pipeline for PHP. The NewQuery-before-Parse
// ordering is mandatory: ts_parser_delete leaves a dlmalloc sentinel in
// the free list that corrupts ts_query_new's internal malloc if called
// after Parse.
func TestPhpQuery_ExtractsKnownSymbols(t *testing.T) {
	ctx := context.Background()
	r, err := treesitter.GetRuntimeForTest(ctx)
	if err != nil {
		t.Skipf("runtime unavailable: %v", err)
	}
	lang, err := r.LoadGrammar(ctx, "php", loadPhpGrammar(t))
	if err != nil {
		t.Fatalf("LoadGrammar: %v", err)
	}
	defer lang.Close(ctx)

	// IMPORTANT: NewQuery must be called BEFORE Parse on this Language.
	q, err := lang.NewQuery(ctx, PHPTags)
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	defer q.Close(ctx)

	src := []byte(`<?php
function login() {}
class AuthService {
    public function authenticate() {}
}
`)
	tree, err := lang.Parse(ctx, src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close(ctx)

	caps, err := q.Exec(ctx, tree.RootNode(ctx))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	syms := ScanCaptures(caps, src, "test.php")
	defSet := map[string]bool{}
	for _, s := range syms {
		if s.Kind == symbols.SymDef {
			defSet[s.Name] = true
		}
	}
	for _, want := range []string{"login", "AuthService", "authenticate"} {
		if !defSet[want] {
			t.Errorf("missing def %q in %v", want, syms)
		}
	}
}
