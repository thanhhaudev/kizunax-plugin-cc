//go:build !lite

package grammars

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstall_HappyPath(t *testing.T) {
	payload := []byte("fake-wasm-bytes")
	sum := sha256.Sum256(payload)
	expectedHash := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write(payload)
	}))
	defer srv.Close()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	entry := Entry{
		Lang: "fake", GrammarName: "fake",
		NpmPackage: "tree-sitter-fake", Version: "1.0.0",
		WasmFile: "fake.wasm", SHA256: expectedHash,
	}

	if err := installFromURL(context.Background(), entry, srv.URL, false); err != nil {
		t.Fatalf("install: %v", err)
	}

	dest := filepath.Join(tmpHome, ".kizunax/grammars/fake.wasm")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected file at %s: %v", dest, err)
	}
	if string(data) != string(payload) {
		t.Fatalf("content mismatch")
	}
}

func TestInstall_SHAMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("wrong payload"))
	}))
	defer srv.Close()

	entry := Entry{
		Lang: "fake", GrammarName: "fake",
		NpmPackage: "tree-sitter-fake", Version: "1.0.0",
		WasmFile: "fake.wasm",
		// 64 hex chars that won't match SHA256("wrong payload")
		SHA256: "deadbeef0000000000000000000000000000000000000000000000000000beef",
	}
	t.Setenv("HOME", t.TempDir())

	if err := installFromURL(context.Background(), entry, srv.URL, false); err == nil {
		t.Fatal("expected SHA mismatch error")
	}
}

func TestInstall_Http404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	entry := Entry{
		Lang: "fake", GrammarName: "fake",
		WasmFile: "fake.wasm", SHA256: "any",
	}
	t.Setenv("HOME", t.TempDir())

	if err := installFromURL(context.Background(), entry, srv.URL, false); err == nil {
		t.Fatal("expected 404 error")
	}
}
