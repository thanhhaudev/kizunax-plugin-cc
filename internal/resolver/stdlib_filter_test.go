package resolver

import (
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols"
)

func TestIsStdlibSymbol_Go(t *testing.T) {
	cases := []struct {
		sym  symbols.Symbol
		want bool
	}{
		{symbols.Symbol{Pkg: "path", Name: "Base", File: "main.go"}, true},
		{symbols.Symbol{Pkg: "fmt", Name: "Println", File: "main.go"}, true},
		{symbols.Symbol{Pkg: "os", Name: "Open", File: "main.go"}, true},
		{symbols.Symbol{Pkg: "myapp", Name: "Foo", File: "main.go"}, false},
		{symbols.Symbol{Name: "Bar", File: "main.go"}, false}, // no pkg → not stdlib
	}
	for _, c := range cases {
		if got := IsStdlibSymbol(c.sym); got != c.want {
			t.Fatalf("sym=%+v: got %v want %v", c.sym, got, c.want)
		}
	}
}

func TestIsStdlibSymbol_Python(t *testing.T) {
	cases := []struct {
		sym  symbols.Symbol
		want bool
	}{
		{symbols.Symbol{Name: "os", Kind: symbols.SymImport, File: "main.py"}, true},
		{symbols.Symbol{Name: "sys", Kind: symbols.SymImport, File: "main.py"}, true},
		{symbols.Symbol{Name: "json", Kind: symbols.SymImport, File: "main.py"}, true},
		{symbols.Symbol{Name: "mypackage", Kind: symbols.SymImport, File: "main.py"}, false},
	}
	for _, c := range cases {
		if got := IsStdlibSymbol(c.sym); got != c.want {
			t.Fatalf("sym=%+v: got %v want %v", c.sym, got, c.want)
		}
	}
}

func TestIsStdlibSymbol_LanguageScoped(t *testing.T) {
	// A Go project package literally named `util` must NOT be filtered
	// just because Node has a util module. v0.12 bug fix.
	goUtil := symbols.Symbol{Pkg: "util", Name: "UnixMillis", Kind: symbols.SymCall, File: "internal/auth/auth.go"}
	if IsStdlibSymbol(goUtil) {
		t.Fatalf("Go package util must NOT be filtered as TypeScript util module")
	}
	// Conversely, a TypeScript util.foo() reference SHOULD be filtered.
	tsUtil := symbols.Symbol{Pkg: "util", Name: "format", Kind: symbols.SymCall, File: "src/index.ts"}
	if !IsStdlibSymbol(tsUtil) {
		t.Fatalf("TypeScript util module SHOULD be filtered as stdlib")
	}
}

func TestIsStdlibSymbol_UnknownExtensionFailsOpen(t *testing.T) {
	// Unknown source language → return false (let resolver search).
	sym := symbols.Symbol{Pkg: "os", Name: "Open", File: "build.gradle"}
	if IsStdlibSymbol(sym) {
		t.Fatalf("unknown ext should fail-open (return false)")
	}
}
