package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
)

// mockProvider returns canned responses in order.
type mockProvider struct {
	responses []provider.ChatResponse
	errs      []error
	calls     int
	lastReq   provider.ChatRequest
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	m.lastReq = req
	idx := m.calls
	m.calls++
	if idx >= len(m.responses) {
		return provider.ChatResponse{}, errors.New("no more mock responses")
	}
	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	return m.responses[idx], err
}

func (m *mockProvider) Probe(ctx context.Context) error { return nil }

// setupPluginRoot creates a temp dir with the prompts and schema files runner.Run needs.
func setupPluginRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "schemas"), 0o755); err != nil {
		t.Fatal(err)
	}
	tmpl := "Target: {{TARGET_LABEL}}\n{{USER_FOCUS}}\nSchema: {{SCHEMA_INLINE}}\nDiff: {{REVIEW_INPUT}}\n"
	if err := os.WriteFile(filepath.Join(root, "prompts", "review.md"), []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prompts", "adversarial-review.md"), []byte("ADVERSARIAL "+tmpl), 0o644); err != nil {
		t.Fatal(err)
	}
	schema := `{"type":"object","properties":{"verdict":{"type":"string"}}}`
	if err := os.WriteFile(filepath.Join(root, "schemas", "review-output.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func sampleBundle() diff.Bundle {
	return diff.Bundle{
		TargetLabel: "test target",
		Diff:        "--- a/file\n+++ b/file\n@@ -1 +1 @@\n-old\n+new",
		TotalBytes:  64,
	}
}

func TestRun_HappyPath(t *testing.T) {
	root := setupPluginRoot(t)
	p := &mockProvider{
		responses: []provider.ChatResponse{
			{
				Content:     `{"verdict":"approve","summary":"clean","findings":[],"next_steps":[]}`,
				StopReason:  "stop",
				InputTokens: 10, OutputTokens: 5, TotalTokens: 15,
			},
		},
	}

	result, err := Run(context.Background(), root, p, sampleBundle(), Options{
		Mode: prompt.ModeStandard, Model: "m", MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Review.Verdict != "approve" {
		t.Errorf("verdict = %q", result.Review.Verdict)
	}
	if result.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d", result.TotalTokens)
	}
	if p.calls != 1 {
		t.Errorf("expected 1 call, got %d", p.calls)
	}
}

func TestRun_ParseRetry_Success(t *testing.T) {
	root := setupPluginRoot(t)
	p := &mockProvider{
		responses: []provider.ChatResponse{
			{Content: "Some prose before...\ngarbage no json", StopReason: "stop"},
			{Content: `{"verdict":"needs-attention","summary":"x","findings":[],"next_steps":[]}`, StopReason: "stop", TotalTokens: 20},
		},
	}

	result, err := Run(context.Background(), root, p, sampleBundle(), Options{
		Mode: prompt.ModeStandard, Model: "m", MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if result.Review.Verdict != "needs-attention" {
		t.Errorf("verdict = %q", result.Review.Verdict)
	}
	if p.calls != 2 {
		t.Errorf("expected 2 calls, got %d", p.calls)
	}
	// Last call should have TryJSONSchema=false (fallback)
	if p.lastReq.TryJSONSchema {
		t.Error("retry should disable TryJSONSchema")
	}
}

func TestRun_ParseRetry_FailGivesUp(t *testing.T) {
	root := setupPluginRoot(t)
	p := &mockProvider{
		responses: []provider.ChatResponse{
			{Content: "not json", StopReason: "stop"},
			{Content: "still not json", StopReason: "stop"},
		},
	}

	_, err := Run(context.Background(), root, p, sampleBundle(), Options{
		Mode: prompt.ModeStandard, Model: "m", MaxTokens: 100,
	})
	if err == nil {
		t.Fatal("expected error after retry failure")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
	if p.calls != 2 {
		t.Errorf("expected 2 calls before giving up, got %d", p.calls)
	}
}

func TestRun_AdversarialMode_LoadsRightTemplate(t *testing.T) {
	root := setupPluginRoot(t)
	p := &mockProvider{
		responses: []provider.ChatResponse{
			{Content: `{"verdict":"approve","summary":"x","findings":[],"next_steps":[]}`, StopReason: "stop"},
		},
	}

	_, err := Run(context.Background(), root, p, sampleBundle(), Options{
		Mode: prompt.ModeAdversarial, Model: "m", MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Verify the adversarial template was loaded by checking sent prompt content.
	if !strings.Contains(p.lastReq.Messages[0].Content, "ADVERSARIAL") {
		t.Errorf("expected adversarial template marker in prompt; got: %s", p.lastReq.Messages[0].Content)
	}
}

func TestRun_FocusInjected(t *testing.T) {
	root := setupPluginRoot(t)
	p := &mockProvider{
		responses: []provider.ChatResponse{
			{Content: `{"verdict":"approve","summary":"x","findings":[],"next_steps":[]}`, StopReason: "stop"},
		},
	}

	_, err := Run(context.Background(), root, p, sampleBundle(), Options{
		Mode:  prompt.ModeStandard,
		Focus: "auth flow",
		Model: "m", MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(p.lastReq.Messages[0].Content, "auth flow") {
		t.Errorf("focus text not in prompt; got: %s", p.lastReq.Messages[0].Content)
	}
}

// suppress unused-import warnings if other helpers added later
var _ = git.TargetWorkingTree
