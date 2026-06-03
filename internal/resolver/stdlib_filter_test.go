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
		{symbols.Symbol{Pkg: "path", Name: "Base"}, true},
		{symbols.Symbol{Pkg: "fmt", Name: "Println"}, true},
		{symbols.Symbol{Pkg: "os", Name: "Open"}, true},
		{symbols.Symbol{Pkg: "myapp", Name: "Foo"}, false},
		{symbols.Symbol{Name: "Bar"}, false}, // no pkg → not stdlib
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
		{symbols.Symbol{Name: "os", Kind: symbols.SymImport}, true},
		{symbols.Symbol{Name: "sys", Kind: symbols.SymImport}, true},
		{symbols.Symbol{Name: "json", Kind: symbols.SymImport}, true},
		{symbols.Symbol{Name: "mypackage", Kind: symbols.SymImport}, false},
	}
	for _, c := range cases {
		if got := IsStdlibSymbol(c.sym); got != c.want {
			t.Fatalf("sym=%+v: got %v want %v", c.sym, got, c.want)
		}
	}
}
