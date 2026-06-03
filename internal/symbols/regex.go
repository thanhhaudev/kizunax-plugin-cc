package symbols

import (
	"bufio"
	"bytes"
	"regexp"
)

// RegexExtractor is the universal fallback extractor.
// Used for any non-Go file when WASM grammar is unavailable
// (either no grammar bundled, or building with -tags lite).
// Patterns are looked up in langPatterns by the lang field
// (set by the factory). An empty lang resolves to "default".
type RegexExtractor struct {
	lang string
}

// patternSet holds the regex patterns used by RegexExtractor for one
// language. Each slice may contain multiple alternates that are tried
// in order; the first match per line wins for defs/imports, and all
// matches across all alternates accumulate for calls.
type patternSet struct {
	defs    []*regexp.Regexp // capture group 1 = symbol name
	imports []*regexp.Regexp // capture group 1 = imported symbol or module
	calls   []*regexp.Regexp // capture group 1 = pkg/receiver, group 2 = method
}

var (
	defRe = regexp.MustCompile(
		`(?:export\s+|public\s+|private\s+|protected\s+|async\s+|abstract\s+)*` +
			`(?:func|fn|def|function|class|struct|type|interface|enum|trait|impl|module|record)` +
			`\s+([A-Za-z_]\w*)`,
	)
	importRe = regexp.MustCompile(
		`(?:^|\s)(?:import|from|use|require|using)\s+(?:\{[^}]+\}\s+from\s+)?["']?([A-Za-z_][\w\./:-]*)["']?`,
	)
	callRe = regexp.MustCompile(
		`\b([A-Za-z_]\w*)\.([A-Za-z_]\w*)\s*\(`,
	)
)

// langPatterns maps a language key (returned by extToLang) to its
// patternSet. The "default" key is the universal fallback used when
// the language is unknown — its contents preserve v0.12.0 behavior.
//
// Add a new language by:
//  1. Add a new map entry here.
//  2. Add the extension → language mapping in extToLang (factory.go).
//  3. Add table-driven test cases in regex_test.go.
var langPatterns = map[string]patternSet{
	"default": {
		defs:    []*regexp.Regexp{defRe},
		imports: []*regexp.Regexp{importRe},
		calls:   []*regexp.Regexp{callRe},
	},
}

func (e *RegexExtractor) Extract(path string, content []byte) []Symbol {
	key := e.lang
	if key == "" {
		key = "default"
	}
	ps, ok := langPatterns[key]
	if !ok {
		ps = langPatterns["default"]
	}

	var syms []Symbol
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()

		for _, re := range ps.defs {
			if m := re.FindSubmatch(line); m != nil {
				syms = append(syms, Symbol{
					Name: string(m[1]),
					Kind: SymDef,
					File: path,
					Line: lineNo,
				})
				break
			}
		}
		for _, re := range ps.imports {
			if m := re.FindSubmatch(line); m != nil {
				syms = append(syms, Symbol{
					Name: string(m[1]),
					Kind: SymImport,
					File: path,
					Line: lineNo,
				})
				break
			}
		}
		for _, re := range ps.calls {
			for _, m := range re.FindAllSubmatch(line, -1) {
				syms = append(syms, Symbol{
					Name: string(m[2]),
					Pkg:  string(m[1]),
					Kind: SymCall,
					File: path,
					Line: lineNo,
				})
			}
		}
	}
	return syms
}
