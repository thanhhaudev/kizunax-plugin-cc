//go:build !lite

package symbols

import (
	"embed"
	"path/filepath"

	// wazero is the pure-Go WebAssembly runtime intended for tree-sitter
	// grammar execution. Imported here so the dependency is pinned in
	// go.mod even before the WASM bridge is wired. Per v0.12.1 scope
	// split, full WASM extraction lands in v0.12.2.
	_ "github.com/tetratelabs/wazero"
)

//go:embed all:grammars
var grammarFS embed.FS

// wasmGrammarNameFor maps a file extension to a tree-sitter grammar
// name. Add entries here when new grammars are bundled. Returns ""
// for extensions not handled by WASM.
var wasmGrammarNameFor = func(ext string) string {
	switch ext {
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".scala":
		return "scala"
	case ".cpp", ".hpp", ".cc", ".hh":
		return "cpp"
	case ".c", ".h":
		return "c"
	}
	return ""
}

// useWASM returns true if a grammar is BUNDLED for ext. (It may not be
// COMPILED yet — see fallback in WASMExtractor.Extract.)
func useWASM(ext string) bool {
	return wasmGrammarNameFor(ext) != ""
}

// newWASMExtractor returns a WASMExtractor configured for ext.
// If the .wasm grammar file is missing at runtime (not yet compiled),
// Extract falls back to regex behavior — strictly additive: enrichment
// works, just less precise.
func newWASMExtractor(ext string) Extractor {
	name := wasmGrammarNameFor(ext)
	return &wasmExtractor{grammarName: name}
}

type wasmExtractor struct {
	grammarName string
}

func (e *wasmExtractor) Extract(path string, content []byte) []Symbol {
	// Try to load grammar from embed.
	data, err := grammarFS.ReadFile(filepath.Join("grammars", e.grammarName+".wasm"))
	if err != nil || len(data) == 0 {
		// Grammar not present (not yet compiled or excluded).
		// Fall back to regex — same precision as -tags lite for this file.
		return (&RegexExtractor{lang: extToLang(filepath.Ext(path))}).Extract(path, content)
	}

	// TODO(v0.12.2): wire wazero runtime + tree-sitter parse here.
	// v0.12.1 keeps the regex fallback in place while per-language
	// regex patterns deliver non-Go enrichment value. Full WASM
	// extraction lands in v0.12.2.
	//
	// To enable real WASM parsing:
	//   1. Set up wazero runtime via sync.Once (memory map grammar).
	//   2. Call exported tree-sitter parse function with content bytes.
	//   3. Walk returned tree, mapping nodes to Symbol structs.
	// See: https://github.com/tree-sitter/tree-sitter/blob/master/lib/binding_web/binding.ts
	return (&RegexExtractor{lang: extToLang(filepath.Ext(path))}).Extract(path, content)
}
