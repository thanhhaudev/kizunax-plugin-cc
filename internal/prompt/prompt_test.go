package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
)

func TestBuild_GlossaryPrepended_WhenNonEmpty(t *testing.T) {
	root := setupFakePluginRoot(t)
	bundle := diff.Bundle{TargetLabel: "test"}
	gloss := "Account = customer account, not bank account"

	p, err := Build(root, ModeStandard, bundle, "{}", "", gloss)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(p.System, "Project glossary") {
		t.Fatalf("expected glossary header in system: %q", p.System)
	}
	if !strings.Contains(p.System, gloss) {
		t.Fatalf("expected glossary content in system: %q", p.System)
	}
	// Existing default system body MUST remain present after glossary.
	if !strings.Contains(p.System, "senior code reviewer") {
		t.Fatalf("default system body lost: %q", p.System)
	}
}

func TestBuild_GlossaryOmitted_WhenEmpty(t *testing.T) {
	root := setupFakePluginRoot(t)
	bundle := diff.Bundle{TargetLabel: "test"}

	p, err := Build(root, ModeStandard, bundle, "{}", "", "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(p.System, "Project glossary") {
		t.Fatalf("glossary header should be absent when empty")
	}
}

func TestBuild_GlossaryAppliesToAdversarial(t *testing.T) {
	root := setupFakePluginRoot(t)
	bundle := diff.Bundle{TargetLabel: "test"}

	p, err := Build(root, ModeAdversarial, bundle, "{}", "", "GLOSSARY")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(p.System, "GLOSSARY") {
		t.Fatalf("expected glossary in adversarial mode too")
	}
}

func setupFakePluginRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "prompts"))
	mustWrite(t, filepath.Join(root, "prompts", "review.md"), "REVIEW: {{TARGET_LABEL}}")
	mustWrite(t, filepath.Join(root, "prompts", "adversarial-review.md"), "ADV: {{TARGET_LABEL}}")
	return root
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, c string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}
