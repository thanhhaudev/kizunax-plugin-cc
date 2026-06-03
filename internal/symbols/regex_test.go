package symbols

import "testing"

func TestRegex_ExtractsTypeScriptFunction(t *testing.T) {
	src := []byte(`
export function authenticate(userId: number): Promise<User> {
	return tokenCache.get(userId);
}
`)
	e := &RegexExtractor{}
	syms := e.Extract("auth.ts", src)
	defs := symbolNames(syms, SymDef)
	wantContains(t, defs, "authenticate")
	calls := symbolNames(syms, SymCall)
	wantContains(t, calls, "get")
}

func TestRegex_ExtractsPythonDef(t *testing.T) {
	src := []byte(`
import os

def authenticate(user_id):
    session = SessionStore.get(user_id)
    return session.user
`)
	e := &RegexExtractor{}
	syms := e.Extract("auth.py", src)
	defs := symbolNames(syms, SymDef)
	wantContains(t, defs, "authenticate")
	imports := symbolNames(syms, SymImport)
	wantContains(t, imports, "os")
}

func TestRegex_ExtractsRustFn(t *testing.T) {
	src := []byte(`
use std::collections::HashMap;

fn build_map() -> HashMap<String, i32> {
	HashMap::new()
}
`)
	e := &RegexExtractor{}
	syms := e.Extract("main.rs", src)
	defs := symbolNames(syms, SymDef)
	wantContains(t, defs, "build_map")
}

func TestRegex_ExtractsJavaClass(t *testing.T) {
	src := []byte(`
public class UserRepository {
	private final Cache<String, User> cache;

	public User findById(String id) {
		return cache.get(id);
	}
}
`)
	e := &RegexExtractor{}
	syms := e.Extract("UserRepository.java", src)
	defs := symbolNames(syms, SymDef)
	wantContains(t, defs, "UserRepository")
}

func TestRegex_ExtractCallSites(t *testing.T) {
	src := []byte(`tokenCache.get(req.userId);`)
	e := &RegexExtractor{}
	syms := e.Extract("file.ts", src)
	for _, s := range syms {
		if s.Kind == SymCall && s.Name == "get" && s.Pkg == "tokenCache" {
			return
		}
	}
	t.Fatalf("expected tokenCache.get call, got %+v", syms)
}

func TestRegex_PositionInfoRecorded(t *testing.T) {
	src := []byte("// header\n\ndef hello():\n    pass\n")
	e := &RegexExtractor{}
	syms := e.Extract("h.py", src)
	for _, s := range syms {
		if s.Kind == SymDef && s.Name == "hello" {
			if s.Line != 3 {
				t.Fatalf("expected Line=3, got %d", s.Line)
			}
			if s.File != "h.py" {
				t.Fatalf("expected File=h.py, got %q", s.File)
			}
			return
		}
	}
	t.Fatalf("hello def not found")
}

func TestLangPatterns_HasDefaultEntry(t *testing.T) {
	ps, ok := langPatterns["default"]
	if !ok {
		t.Fatal("langPatterns[\"default\"] must exist as the fallback")
	}
	if len(ps.defs) == 0 || len(ps.imports) == 0 || len(ps.calls) == 0 {
		t.Fatalf("default patternSet must have defs/imports/calls populated: %+v", ps)
	}
}
