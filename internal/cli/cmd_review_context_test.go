package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	llmcontext "github.com/thanhhaudev/llmreviewkit/context"
)

// These tests validate that llmcontext.Load behaves as cmd_review.go expects:
// missing file → empty Path + zero ModTime; existing file → populated.

func TestLLMContext_Load_MissingFile(t *testing.T) {
	dir := t.TempDir()
	c, err := llmcontext.Load(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c.Path != "" {
		t.Errorf("expected empty Path, got %q", c.Path)
	}
	if !c.ModTime.IsZero() {
		t.Errorf("expected zero ModTime, got %v", c.ModTime)
	}
}

func TestLLMContext_Load_StaleDetection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "REVIEW-CONTEXT.md")
	if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to 20 days ago.
	old := time.Now().Add(-20 * 24 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}

	c, _ := llmcontext.Load(dir)
	if c.ModTime.IsZero() {
		t.Fatalf("expected non-zero ModTime")
	}
	age := time.Since(c.ModTime)
	if age < 14*24*time.Hour {
		t.Errorf("expected age >= 14 days, got %v", age)
	}
}
