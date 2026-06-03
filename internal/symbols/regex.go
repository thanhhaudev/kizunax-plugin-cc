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
	"php": {
		defs: []*regexp.Regexp{
			// function (with optional visibility/abstract/static/final)
			regexp.MustCompile(`(?:public\s+|private\s+|protected\s+|static\s+|final\s+|abstract\s+)*function\s+([A-Za-z_]\w*)`),
			// class / interface / trait / enum
			regexp.MustCompile(`(?:abstract\s+|final\s+)*(?:class|interface|trait|enum)\s+([A-Za-z_]\w*)`),
		},
		imports: []*regexp.Regexp{
			// use App\Foo\Bar;          → capture "Bar"
			// use App\Foo\Bar as Baz;   → capture "Baz" via the alias group
			regexp.MustCompile(`\buse\s+(?:function\s+|const\s+)?(?:\\?[A-Za-z_]\w*(?:\\[A-Za-z_]\w*)*\\)?([A-Za-z_]\w*)(?:\s+as\s+([A-Za-z_]\w*))?\s*;`),
		},
		calls: []*regexp.Regexp{
			// Class::method(
			regexp.MustCompile(`\b([A-Za-z_]\w*)::([A-Za-z_]\w*)\s*\(`),
			// $obj->method( — receiver var + method
			regexp.MustCompile(`\$([A-Za-z_]\w*)->([A-Za-z_]\w*)\s*\(`),
			// ->intermediate->method( — chained terminal ($this->db->fetchRow())
			regexp.MustCompile(`->([A-Za-z_]\w*)->([A-Za-z_]\w*)\s*\(`),
		},
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
			m := re.FindSubmatch(line)
			if m == nil {
				continue
			}
			// Pick the last non-empty capture group so patterns with an
			// optional alias group (e.g. PHP "use X as Y") emit "Y"
			// while single-group patterns still emit group 1.
			name := ""
			for i := len(m) - 1; i >= 1; i-- {
				if len(m[i]) > 0 {
					name = string(m[i])
					break
				}
			}
			if name == "" {
				break
			}
			syms = append(syms, Symbol{
				Name: name,
				Kind: SymImport,
				File: path,
				Line: lineNo,
			})
			break
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
