package helper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/llmreviewkit/schema"
)

func TestSummarize_EmptyFindings_ShortCircuit(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	cfg := config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 5}
	out, err := Summarize(context.Background(), cfg, "kx_test", schema.ReviewResult{Verdict: "approve"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty TL;DR for empty findings, got %q", out)
	}
	if called {
		t.Fatalf("HTTP must not be called when findings is empty")
	}
}

func TestSummarize_RequestShape(t *testing.T) {
	var gotBody Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"3 issues; race condition is critical."}}]}`))
	}))
	defer srv.Close()

	cfg := config.HelperConfig{BaseURL: srv.URL, Model: "qwen3.5-flash", TimeoutSeconds: 5}
	result := schema.ReviewResult{
		Verdict: "needs-attention",
		Findings: []schema.Finding{
			{Severity: "critical", Title: "Race condition", File: "a.go", LineStart: 1, LineEnd: 2, Body: "Race in refresh"},
			{Severity: "high", Title: "Missing error wrap", File: "b.go", LineStart: 5, LineEnd: 5, Body: "Lost cause"},
			{Severity: "medium", Title: "Sort inefficient", File: "c.go", LineStart: 9, LineEnd: 9, Body: "n log n possible"},
		},
	}
	out, err := Summarize(context.Background(), cfg, "kx_test", result)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if !strings.Contains(out, "3 issues") {
		t.Fatalf("unexpected TL;DR: %q", out)
	}
	if gotBody.MaxTokens != 300 {
		t.Fatalf("expected max_tokens=300, got %d", gotBody.MaxTokens)
	}
	if gotBody.Temperature != 0.2 {
		t.Fatalf("expected temperature=0.2, got %v", gotBody.Temperature)
	}
	if gotBody.Model != "qwen3.5-flash" {
		t.Fatalf("expected model=qwen3.5-flash, got %q", gotBody.Model)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" {
		t.Fatalf("bad messages: %+v", gotBody.Messages)
	}
	if !strings.Contains(gotBody.Messages[1].Content, "Race condition") {
		t.Fatalf("user message missing first finding title: %q", gotBody.Messages[1].Content)
	}
}

func TestSummarize_FindingsCappedAt2KiB(t *testing.T) {
	var gotBody Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	cfg := config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 5}
	big := strings.Repeat("x", 5000)
	result := schema.ReviewResult{
		Verdict: "needs-attention",
		Findings: []schema.Finding{
			{Severity: "critical", Title: "Title", File: "a.go", LineStart: 1, LineEnd: 1, Body: big},
		},
	}
	if _, err := Summarize(context.Background(), cfg, "kx", result); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := len(gotBody.Messages[1].Content); got > maxSummarizeInputBytes+200 {
		t.Fatalf("user message %d bytes exceeds cap %d (+slack)", got, maxSummarizeInputBytes)
	}
}

func TestSummarize_TruncationKeepsValidUTF8(t *testing.T) {
	var gotBody Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	// 3-byte rune "ế" repeated past both the per-finding 240-byte cap and
	// the 2KiB total cap. Without rune-safe truncation, byte-slicing would
	// produce invalid UTF-8 mid-rune.
	bigVN := strings.Repeat("ế", 1000)
	cfg := config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 5}
	result := schema.ReviewResult{
		Verdict: "needs-attention",
		Findings: []schema.Finding{
			{Severity: "critical", Title: "Title", File: "a.go", LineStart: 1, LineEnd: 1, Body: bigVN},
		},
	}
	if _, err := Summarize(context.Background(), cfg, "kx", result); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !utf8.ValidString(gotBody.Messages[1].Content) {
		t.Fatalf("serialized findings contain invalid UTF-8")
	}
}

func TestSummarize_TimeoutEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond)
	}))
	defer srv.Close()

	cfg := config.HelperConfig{BaseURL: srv.URL, Model: "m", TimeoutSeconds: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := schema.ReviewResult{
		Verdict:  "needs-attention",
		Findings: []schema.Finding{{Severity: "low", Title: "T", File: "f", LineStart: 1, LineEnd: 1}},
	}
	if _, err := Summarize(ctx, cfg, "kx", result); err == nil {
		t.Fatalf("expected timeout error")
	}
}
