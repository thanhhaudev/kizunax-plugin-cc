//go:build !lite

package symbols

// useWASM stub — replaced with real implementation in Task 13 (wasm.go).
// Returns false here so factory.go falls through to RegexExtractor.
// IMPORTANT: this file is deleted (or function moved to wasm.go) once
// Task 13 ships. Until then, default build behaves like lite build.
func useWASM(ext string) bool { return false }

func newWASMExtractor(ext string) Extractor { return &RegexExtractor{} }
