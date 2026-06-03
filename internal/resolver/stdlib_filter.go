package resolver

import (
	"path/filepath"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/symbols"
)

// Hardcoded stdlib package skip lists per language. Symbols whose Pkg
// matches a stdlib package are skipped — the LLM already knows them.
//
// Coverage focuses on the most-referenced stdlib packages; extending each
// list is an ongoing follow-up.

var goStdlibPkgs = map[string]bool{
	"fmt": true, "os": true, "io": true, "ioutil": true,
	"strings": true, "strconv": true, "bytes": true, "bufio": true,
	"errors": true, "context": true, "time": true,
	"sync": true, "atomic": true,
	"path": true, "filepath": true,
	"regexp":        true,
	"encoding/json": true, "encoding/hex": true, "encoding/base64": true,
	"encoding/binary": true, "encoding/csv": true,
	"net/http": true, "net/url": true, "net": true,
	"go/ast": true, "go/parser": true, "go/token": true, "go/types": true,
	"reflect": true,
	"unicode": true, "unicode/utf8": true,
	"sort": true,
	"math": true, "math/rand": true, "math/big": true,
	"hash": true, "hash/sha256": true, "hash/sha1": true,
	"crypto": true, "crypto/sha256": true, "crypto/rand": true, "crypto/tls": true,
	"log": true, "log/slog": true,
	"flag":    true,
	"runtime": true, "runtime/debug": true,
	"testing": true,
	"embed":   true,
}

var pythonStdlibPkgs = map[string]bool{
	"os": true, "sys": true, "io": true, "json": true, "yaml": true,
	"re": true, "typing": true, "collections": true, "itertools": true,
	"functools": true, "datetime": true, "time": true, "uuid": true,
	"pathlib": true, "subprocess": true, "logging": true,
	"asyncio": true, "concurrent": true, "threading": true,
	"http": true, "urllib": true, "socket": true,
	"unittest": true, "pytest": true,
	"abc": true, "dataclasses": true, "enum": true,
}

var tsStdlibPkgs = map[string]bool{
	"fs": true, "path": true, "os": true, "http": true, "https": true,
	"net": true, "url": true, "util": true, "crypto": true,
	"stream": true, "events": true, "buffer": true, "child_process": true,
	"process": true, "console": true,
	"react": true, "vue": true, "@angular/core": true, // common framework imports
}

// IsStdlibSymbol returns true if sym refers to a known stdlib package
// for the source language of sym.File. Language-scoped to prevent
// false collisions across ecosystems — e.g. a Go project package named
// `util` would otherwise be filtered by the Node `util` module entry.
// When sym.File has an unknown extension, returns false (fail-open so
// resolver can search for a definition).
func IsStdlibSymbol(sym symbols.Symbol) bool {
	var pkgs map[string]bool
	switch filepath.Ext(sym.File) {
	case ".go":
		pkgs = goStdlibPkgs
	case ".py":
		pkgs = pythonStdlibPkgs
	case ".ts", ".tsx", ".js", ".jsx", ".mjs":
		pkgs = tsStdlibPkgs
	default:
		return false
	}
	if sym.Pkg != "" && pkgs[sym.Pkg] {
		return true
	}
	if sym.Kind == symbols.SymImport && pkgs[sym.Name] {
		return true
	}
	return false
}
