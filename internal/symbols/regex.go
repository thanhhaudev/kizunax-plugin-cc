package symbols

import (
	"bufio"
	"bytes"
	"regexp"
)

// RegexExtractor is the universal fallback extractor.
// Used for any non-Go file when WASM grammar is unavailable
// (either no grammar bundled, or building with -tags lite).
// Patterns are intentionally permissive — false positives at extraction
// are filtered later by the resolver (a non-existent symbol simply yields
// zero references during the workspace grep).
type RegexExtractor struct{}

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

func (e *RegexExtractor) Extract(path string, content []byte) []Symbol {
	var syms []Symbol
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()

		if m := defRe.FindSubmatch(line); m != nil {
			syms = append(syms, Symbol{
				Name: string(m[1]),
				Kind: SymDef,
				File: path,
				Line: lineNo,
			})
		}
		if m := importRe.FindSubmatch(line); m != nil {
			syms = append(syms, Symbol{
				Name: string(m[1]),
				Kind: SymImport,
				File: path,
				Line: lineNo,
			})
		}
		for _, m := range callRe.FindAllSubmatch(line, -1) {
			syms = append(syms, Symbol{
				Name: string(m[2]),
				Pkg:  string(m[1]),
				Kind: SymCall,
				File: path,
				Line: lineNo,
			})
		}
	}
	return syms
}
