//go:build !lite

package symbols

import "testing"

func TestUseWASM_ReturnsTrueForKnownGrammar(t *testing.T) {
	// Even without compiled grammars present, useWASM should report which
	// extensions WOULD use WASM. (The actual WASM call falls back to regex
	// when grammar file is missing.)
	cases := []string{".ts", ".tsx", ".py", ".rs", ".java"}
	for _, ext := range cases {
		if !useWASM(ext) {
			t.Fatalf("expected useWASM(%q)=true (default build, grammar bundled or stubbed), got false", ext)
		}
	}
}

func TestUseWASM_ReturnsFalseForUnknownExt(t *testing.T) {
	if useWASM(".unknown") {
		t.Fatalf("expected useWASM(unknown)=false")
	}
}

func TestWASMExtractor_FallsBackWhenGrammarMissing(t *testing.T) {
	// Grammar files not yet committed (Task 14 compiles them). The extractor
	// must gracefully fall back to regex behavior — never panic, never block.
	e := newWASMExtractor(".ts")
	syms := e.Extract("auth.ts", []byte(`export function authenticate() {}`))
	// Should return SOME symbols (via regex fallback) even with no grammar.
	if len(syms) == 0 {
		t.Fatalf("expected fallback regex to extract symbols when grammar missing")
	}
}
