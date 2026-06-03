//go:build !lite

package symbols

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols/treesitter"
)

//go:embed all:grammars
var grammarFS embed.FS

// workspaceRoot is set by the runner before bundle extraction so the
// resolver knows where to look for project-local grammars.
var (
	workspaceRoot   string
	workspaceRootMu sync.Mutex
)

// SetWorkspaceRoot configures the project root used for project-local
// grammar lookup (./.kizunax/grammars/). Must be called by the runner
// before ExtractFromBundle. Safe to call concurrently.
func SetWorkspaceRoot(ws string) {
	workspaceRootMu.Lock()
	defer workspaceRootMu.Unlock()
	workspaceRoot = ws
}

func getWorkspaceRoot() string {
	workspaceRootMu.Lock()
	defer workspaceRootMu.Unlock()
	return workspaceRoot
}

// hintEmitted tracks per-grammar verbose hints to avoid log spam.
var (
	hintEmitted = map[string]bool{}
	hintMu      sync.Mutex
)

func emitHintOnce(grammarName string) {
	hintMu.Lock()
	defer hintMu.Unlock()
	if hintEmitted[grammarName] {
		return
	}
	hintEmitted[grammarName] = true
	fmt.Fprintf(os.Stderr,
		"[verbose] grammars: no .wasm found for %s — try 'kizunax grammars install %s'\n",
		grammarName, grammarName)
}

// langCache caches loaded grammars across extractor instances so each
// grammar only loads once per session.
var (
	langCache   = map[string]*treesitter.Language{}
	langCacheMu sync.Mutex
)

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
	ctx := context.Background()

	// Resolve grammar path via project-local then global lookup.
	resolver := DefaultResolver(getWorkspaceRoot())
	grammarPath := resolver.Find(e.grammarName)
	if grammarPath == "" {
		// No compiled .wasm found — emit a once-per-session verbose hint
		// and fall back to regex so enrichment is still useful.
		emitHintOnce(e.grammarName)
		return regexFallback(path, content)
	}

	lang, err := getCachedLang(ctx, e.grammarName, grammarPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] treesitter load %s: %v\n", e.grammarName, err)
		return regexFallback(path, content)
	}

	queryStr := queryForGrammar(e.grammarName)
	if queryStr == "" {
		// No tags.scm shipped for this grammar yet (typescript/python land
		// in Tasks 17/18; other grammars pending compilation in Task 14).
		return regexFallback(path, content)
	}

	// IMPORTANT: NewQuery must be called BEFORE Parse — see Task 8 finding.
	q, err := lang.NewQuery(ctx, queryStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] treesitter query %s: %v\n", e.grammarName, err)
		return regexFallback(path, content)
	}
	defer q.Close(ctx)

	tree, err := lang.Parse(ctx, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] treesitter parse %s: %v\n", path, err)
		return regexFallback(path, content)
	}
	defer tree.Close(ctx)

	caps, err := q.Exec(ctx, tree.RootNode(ctx))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] treesitter exec %s: %v\n", e.grammarName, err)
		return regexFallback(path, content)
	}

	return scanCaptures(caps, content, path)
}

// regexFallback delegates to RegexExtractor for the given file, providing
// the same symbol precision as the -tags lite build.
func regexFallback(path string, content []byte) []Symbol {
	return (&RegexExtractor{lang: extToLang(filepath.Ext(path))}).Extract(path, content)
}

// getCachedLang returns a cached Language for the given grammar path,
// loading it via the treesitter runtime if not yet cached.
func getCachedLang(ctx context.Context, name, path string) (*treesitter.Language, error) {
	langCacheMu.Lock()
	defer langCacheMu.Unlock()
	if lang, ok := langCache[path]; ok {
		return lang, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read grammar %s: %w", path, err)
	}
	rt, err := treesitter.GetRuntimeForTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	lang, err := rt.LoadGrammar(ctx, name, data)
	if err != nil {
		return nil, fmt.Errorf("load grammar: %w", err)
	}
	langCache[path] = lang
	return lang, nil
}

// phpTags is the tags.scm query for PHP (mirrors queries.PHPTags).
// Inlined here to avoid an import cycle: symbols → symbols/queries → symbols.
const phpTags = `
(function_definition
  name: (name) @name.definition.function)

(method_declaration
  name: (name) @name.definition.method)

(class_declaration
  name: (name) @name.definition.class)

(interface_declaration
  name: (name) @name.definition.interface)

(trait_declaration
  name: (name) @name.definition.trait)

(scoped_call_expression
  scope: (name) @receiver
  name: (name) @name.reference.call)

(nullsafe_member_call_expression
  name: (name) @name.reference.call)

(namespace_use_clause
  (qualified_name (name) @name.reference.import))

(namespace_use_clause
  (name) @name.reference.import)
`

// queryForGrammar returns the tags.scm query string for the given grammar
// name. Returns "" for grammars not yet wired (typescript/python in Tasks
// 17/18; others pending).
func queryForGrammar(name string) string {
	switch name {
	case "php":
		return phpTags
		// "typescript" and "python" land in Tasks 17/18.
	}
	return ""
}

// scanCaptures translates treesitter captures to Symbols. Inlined from
// queries.ScanCaptures to avoid an import cycle (symbols → queries → symbols).
func scanCaptures(captures []treesitter.Capture, src []byte, path string) []Symbol {
	out := make([]Symbol, 0, len(captures))
	for _, c := range captures {
		if c.StartByte >= c.EndByte || int(c.EndByte) > len(src) {
			continue
		}
		name := string(src[c.StartByte:c.EndByte])
		var kind SymbolKind
		switch {
		case strings.HasPrefix(c.Name, "name.definition."):
			kind = SymDef
		case c.Name == "name.reference.call":
			kind = SymCall
		case c.Name == "name.reference.import" || c.Name == "module":
			kind = SymImport
		case c.Name == "name.reference.type":
			kind = SymTypeRef
		default:
			continue
		}
		line := lineAt(src, c.StartByte)
		out = append(out, Symbol{Name: name, Kind: kind, File: path, Line: line})
	}
	return out
}

// lineAt counts newlines in src up to byteOff to compute a 1-based line number.
func lineAt(src []byte, byteOff uint32) int {
	if int(byteOff) > len(src) {
		byteOff = uint32(len(src))
	}
	line := 1
	for i := uint32(0); i < byteOff; i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}
