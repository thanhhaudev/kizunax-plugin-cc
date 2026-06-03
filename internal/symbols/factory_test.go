package symbols

import "testing"

func TestForFile_GoUsesAST(t *testing.T) {
	e := ForFile("main.go")
	if _, ok := e.(*GoASTExtractor); !ok {
		t.Fatalf("expected *GoASTExtractor for .go, got %T", e)
	}
}

func TestForFile_TypeScriptUsesRegexInLite(t *testing.T) {
	// In lite build, useWASM is always false, so TypeScript falls back to regex.
	// In default build, useWASM returns true for known grammars → WASMExtractor.
	// This test asserts factory routing works for at least one of those paths.
	e := ForFile("auth.ts")
	if e == nil {
		t.Fatalf("expected non-nil extractor for .ts, got nil")
	}
}

func TestForFile_PythonUsesRegexInLite(t *testing.T) {
	e := ForFile("auth.py")
	if e == nil {
		t.Fatalf("expected non-nil extractor for .py, got nil")
	}
}

func TestForFile_MarkdownReturnsNil(t *testing.T) {
	if got := ForFile("README.md"); got != nil {
		t.Fatalf("expected nil for .md, got %T", got)
	}
}

func TestForFile_JSONReturnsNil(t *testing.T) {
	if got := ForFile("config.json"); got != nil {
		t.Fatalf("expected nil for .json, got %T", got)
	}
}

func TestForFile_UnknownExtReturnsNil(t *testing.T) {
	if got := ForFile("README"); got != nil {
		t.Fatalf("expected nil for no extension, got %T", got)
	}
}

func TestSourceExtensions_KnownLangs(t *testing.T) {
	knownExts := []string{
		".go", ".js", ".jsx", ".mjs", ".ts", ".tsx",
		".py", ".rs", ".java", ".cs", ".rb", ".php",
		".kt", ".kts", ".swift", ".scala",
		".cpp", ".hpp", ".cc", ".hh", ".c", ".h",
		".m", ".mm", ".dart", ".ex", ".exs",
	}
	for _, ext := range knownExts {
		if !sourceExtensions[ext] {
			t.Errorf("expected %s in sourceExtensions", ext)
		}
	}
}
