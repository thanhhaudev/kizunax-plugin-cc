package helper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChat_HappyPath(t *testing.T) {
	var gotBody Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer kx_test" {
			t.Errorf("Authorization: got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "TL;DR text"}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Model: "qwen3.5-flash", APIKey: "kx_test", HTTPC: srv.Client()}
	out, err := c.Chat(context.Background(), "sys", "usr", 300)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if out != "TL;DR text" {
		t.Fatalf("content: got %q", out)
	}
	if gotBody.Model != "qwen3.5-flash" {
		t.Fatalf("model: got %q", gotBody.Model)
	}
	if gotBody.MaxTokens != 300 {
		t.Fatalf("max_tokens: got %d", gotBody.MaxTokens)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("messages len: got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "sys" {
		t.Fatalf("system msg: %+v", gotBody.Messages[0])
	}
}

func TestChat_NestedErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request","message":"bad payload"}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Model: "m", APIKey: "k", HTTPC: srv.Client()}
	_, err := c.Chat(context.Background(), "s", "u", 100)
	if err == nil || !strings.Contains(err.Error(), "bad payload") {
		t.Fatalf("expected error containing 'bad payload', got %v", err)
	}
}

func TestChat_FlatErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Model: "m", APIKey: "k", HTTPC: srv.Client()}
	_, err := c.Chat(context.Background(), "s", "u", 100)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected error containing 'rate limited', got %v", err)
	}
}

func TestChat_ContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"late"}}]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Model: "m", APIKey: "k", HTTPC: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := c.Chat(ctx, "s", "u", 100)
	if err == nil {
		t.Fatalf("expected context deadline error")
	}
}

func TestChat_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Model: "m", APIKey: "k", HTTPC: srv.Client()}
	_, err := c.Chat(context.Background(), "s", "u", 100)
	if err == nil {
		t.Fatalf("expected error on empty choices")
	}
}
