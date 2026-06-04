// Package index provides a per-workspace AST index that maps symbol names
// to their definition/reference locations across the workspace. Used by
// resolver v2 (internal/resolver/v2.go) to replace regex grep with O(1)
// lookups. Persisted to ~/.kizunax/state/{ws-hash}/index/index.json.
package index

// CurrentSchemaVersion is bumped on any breaking change to the persisted
// format. When LoadJSON encounters a version != Current, the index is
// rebuilt from scratch.
const CurrentSchemaVersion = 1

// Kind classifies a symbol occurrence. Mirrors symbols.SymbolKind but kept
// local to avoid the import cycle (symbols imports diff, diff imports
// nothing that touches index).
type Kind int

const (
	SymDef Kind = iota
	SymCall
	SymImport
	SymTypeRef
)

func (k Kind) String() string {
	switch k {
	case SymDef:
		return "def"
	case SymCall:
		return "call"
	case SymImport:
		return "import"
	case SymTypeRef:
		return "type_ref"
	default:
		return "unknown"
	}
}

// Location is one occurrence of a symbol in the workspace.
type Location struct {
	File     string `json:"file"`               // repo-relative
	Line     int    `json:"line"`
	Kind     Kind   `json:"kind"`
	Pkg      string `json:"pkg,omitempty"`      // e.g. "app.db" for Python qualified
	Receiver string `json:"recv,omitempty"`     // e.g. "*AuthService" for Go method
}

// FileIndex aggregates everything we know about one file.
type FileIndex struct {
	Path    string     `json:"path"`
	Lang    string     `json:"lang"`            // "go" | "python" | "php" | "typescript"
	Mtime   int64      `json:"mtime"`           // unix nanos at last scan
	Hash    string     `json:"hash,omitempty"`  // optional sha256-8 for rename detection
	Defs    []Location `json:"defs"`
	Refs    []Location `json:"refs"`
	Imports []string   `json:"imports"`
}

// Index is the per-workspace global view.
type Index struct {
	Version int                   `json:"v"`
	Root    string                `json:"root"`
	Built   int64                 `json:"built"`  // unix nanos of last full build
	Files   map[string]*FileIndex `json:"files"`
	// Derived on Load; not serialized.
	bySymbol map[string][]Location `json:"-"`
}
