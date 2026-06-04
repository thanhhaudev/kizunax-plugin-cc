package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/git"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/prompt"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/schema"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// mockProvider returns canned responses in order.
type mockProvider struct {
	responses []provider.ChatResponse
	errs      []error
	calls     int
	lastReq   provider.ChatRequest
	requests  []provider.ChatRequest // captured for assertion
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	m.requests = append(m.requests, req)
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
	tmpl := "Target: {{TARGET_LABEL}}\n{{USER_FOCUS}}\nSchema: {{SCHEMA_INLINE}}\nDiff: {{REVIEW_INPUT}}\n{{REFERENCED_FILES}}\n"
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

func TestShouldSummarize(t *testing.T) {
	cases := []struct {
		name     string
		opts     Options
		count    int
		expected bool
	}{
		{"no findings, default", Options{}, 0, false},
		{"1 finding, default", Options{}, 1, false},
		{"2 findings, default", Options{}, 2, false},
		{"3 findings, default", Options{}, 3, true},
		{"5 findings, default", Options{}, 5, true},
		{"NoSummary wins over count", Options{NoSummary: true}, 5, false},
		{"Summary forces on 1 finding", Options{Summary: true}, 1, true},
		{"Summary forces on 0 findings", Options{Summary: true}, 0, true},
		{"NoSummary beats Summary", Options{Summary: true, NoSummary: true}, 5, false},
		{"NoSummary at 0 findings", Options{NoSummary: true}, 0, false},
		{"Summary at 2 findings", Options{Summary: true}, 2, true},
		{"default at 3 findings", Options{}, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := make([]schema.Finding, c.count)
			if got := shouldSummarize(c.opts, findings); got != c.expected {
				t.Fatalf("got %v want %v", got, c.expected)
			}
		})
	}
}

func TestRun_HelperCalled_WhenGated(t *testing.T) {
	var helperCalled int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&helperCalled, 1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"executive tl;dr"}}]}`))
	}))
	defer srv.Close()

	pluginRoot := setupPluginRoot(t)
	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"needs-attention","summary":"s","findings":[
			{"severity":"critical","title":"x","body":"b","file":"f","line_start":1,"line_end":1,"confidence":0.5,"recommendation":"r"},
			{"severity":"high","title":"y","body":"b","file":"f","line_start":2,"line_end":2,"confidence":0.5,"recommendation":"r"},
			{"severity":"medium","title":"z","body":"b","file":"f","line_start":3,"line_end":3,"confidence":0.5,"recommendation":"r"}
		],"next_steps":[]}`,
		},
	}}

	opts := Options{
		Mode:         prompt.ModeStandard,
		Model:        "MiniMax-M2.7-highspeed",
		HelperCfg:    config.HelperConfig{BaseURL: srv.URL, Model: "qwen3.5-flash", TimeoutSeconds: 5},
		HelperAPIKey: "kx_test",
	}
	res, err := Run(context.Background(), pluginRoot, p, diff.Bundle{TargetLabel: "t"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if atomic.LoadInt32(&helperCalled) != 1 {
		t.Fatalf("expected helper called once, got %d", helperCalled)
	}
	if res.Review.TLDR != "executive tl;dr" {
		t.Fatalf("expected TLDR populated, got %q", res.Review.TLDR)
	}
}

func TestRun_HelperSkipped_WhenNoSummary(t *testing.T) {
	var helperCalled int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&helperCalled, 1)
	}))
	defer srv.Close()

	pluginRoot := setupPluginRoot(t)
	// 3 findings so the count-based default would normally fire the helper;
	// only NoSummary should stop it.
	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"needs-attention","summary":"s","findings":[
			{"severity":"critical","title":"x","body":"b","file":"f","line_start":1,"line_end":1,"confidence":0.5,"recommendation":"r"},
			{"severity":"high","title":"y","body":"b","file":"f","line_start":2,"line_end":2,"confidence":0.5,"recommendation":"r"},
			{"severity":"medium","title":"z","body":"b","file":"f","line_start":3,"line_end":3,"confidence":0.5,"recommendation":"r"}
		],"next_steps":[]}`},
	}}

	opts := Options{
		Mode:         prompt.ModeStandard,
		Model:        "MiniMax-M2.7-highspeed",
		NoSummary:    true,
		HelperCfg:    config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 5},
		HelperAPIKey: "kx_test",
	}
	if _, err := Run(context.Background(), pluginRoot, p, diff.Bundle{TargetLabel: "t"}, opts); err != nil {
		t.Fatalf("run: %v", err)
	}
	if atomic.LoadInt32(&helperCalled) != 0 {
		t.Fatalf("helper must not be called with NoSummary, got %d", helperCalled)
	}
}

func TestRun_HelperError_TLDRStaysEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"message":"upstream down"}`))
	}))
	defer srv.Close()

	pluginRoot := setupPluginRoot(t)
	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"needs-attention","summary":"","findings":[
			{"severity":"critical","title":"x","body":"b","file":"f","line_start":1,"line_end":1,"confidence":0.5,"recommendation":"r"},
			{"severity":"high","title":"y","body":"b","file":"f","line_start":2,"line_end":2,"confidence":0.5,"recommendation":"r"},
			{"severity":"low","title":"z","body":"b","file":"f","line_start":3,"line_end":3,"confidence":0.5,"recommendation":"r"}
		],"next_steps":[]}`,
		},
	}}

	opts := Options{
		Mode:         prompt.ModeStandard,
		Model:        "MiniMax-M2.7-highspeed",
		HelperCfg:    config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 5},
		HelperAPIKey: "kx_test",
	}
	res, err := Run(context.Background(), pluginRoot, p, diff.Bundle{TargetLabel: "t"}, opts)
	if err != nil {
		t.Fatalf("helper failure must NOT fail Run: %v", err)
	}
	if res.Review.TLDR != "" {
		t.Fatalf("TLDR must be empty after helper error, got %q", res.Review.TLDR)
	}
}

func TestRun_CanonicalizesBasenameFromDiff(t *testing.T) {
	pluginRoot := setupPluginRoot(t)
	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"needs-attention","summary":"s","findings":[
			{"severity":"critical","title":"race","body":"b","file":"auth.go","line_start":35,"line_end":36,"confidence":0.9,"recommendation":"r"}
		],"next_steps":[]}`},
	}}

	bundle := diff.Bundle{
		TargetLabel: "test",
		Diff: `diff --git a/internal/api/auth.go b/internal/api/auth.go
index abc..def 100644
--- a/internal/api/auth.go
+++ b/internal/api/auth.go
@@ -1,1 +1,1 @@
-old
+new
`,
	}
	res, err := Run(context.Background(), pluginRoot, p, bundle, Options{Mode: prompt.ModeStandard, Model: "m"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Review.Findings[0].File != "internal/api/auth.go" {
		t.Fatalf("expected basename auth.go canonicalized to full path, got %q", res.Review.Findings[0].File)
	}
}

func TestRun_LeavesAmbiguousBasenameAlone(t *testing.T) {
	pluginRoot := setupPluginRoot(t)
	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"needs-attention","summary":"s","findings":[
			{"severity":"critical","title":"race","body":"b","file":"auth.go","line_start":1,"line_end":1,"confidence":0.9,"recommendation":"r"}
		],"next_steps":[]}`},
	}}

	bundle := diff.Bundle{
		TargetLabel: "test",
		Diff: `+++ b/internal/api/auth.go
+++ b/internal/admin/auth.go
`,
	}
	res, err := Run(context.Background(), pluginRoot, p, bundle, Options{Mode: prompt.ModeStandard, Model: "m"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Review.Findings[0].File != "auth.go" {
		t.Fatalf("ambiguous basename should stay, got %q", res.Review.Findings[0].File)
	}
}

func TestRun_EnrichesBundleWithReferencedFiles(t *testing.T) {
	// Workspace fixture: definition in a sibling package.
	// Use "authz" as the package name — not in any stdlib filter list.
	ws := t.TempDir()
	mustWrite := func(p, c string) {
		path := filepath.Join(ws, p)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("authz/checker.go", "package authz\nfunc CheckPerm(role string) bool { return role == \"admin\" }\n")

	pluginRoot := setupPluginRoot(t)
	bundle := diff.Bundle{
		TargetLabel: "test",
		Diff: `diff --git a/main.go b/main.go
+++ b/main.go
@@ -1,1 +1,3 @@
 package main
+import "authz"
+func main() { _ = authz.CheckPerm("admin") }
`,
	}

	p := &mockProvider{responses: []provider.ChatResponse{
		{Content: `{"verdict":"approve","summary":"","findings":[],"next_steps":[]}`},
	}}

	opts := Options{
		Mode:          prompt.ModeStandard,
		Model:         "m",
		WorkspaceRoot: ws,
	}
	_, err := Run(context.Background(), pluginRoot, p, bundle, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Verify the provider received a prompt containing the referenced file.
	if len(p.requests) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(p.requests))
	}
	reqUser := p.requests[0].Messages[0].Content
	if !strings.Contains(reqUser, "authz/checker.go") {
		t.Fatalf("expected referenced file in prompt; got:\n%s", reqUser)
	}
}

// TestLoadIndexForReview_EmptyWorkspaceReturnsNil verifies that
// loadIndexForReview returns nil+error immediately when workspace dir or root
// is empty — the guard at the top of the function. No subprocess is spawned.
func TestLoadIndexForReview_EmptyWorkspaceReturnsNil(t *testing.T) {
	idx, err := loadIndexForReview(state.WorkspaceDir{}, "", false)
	if idx != nil {
		t.Fatalf("expected nil index when workspace is empty, got %+v", idx)
	}
	if err == nil {
		t.Fatal("expected error when workspace is empty")
	}
}

// suppress unused-import warnings if other helpers added later
var _ = git.TargetWorkingTree
