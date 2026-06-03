package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTPC   *http.Client
}

// Chat sends a single-turn completion request. Returns the assistant message
// content. Errors on transport failure, non-2xx response, or empty choices.
func (c *Client) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	reqBody := Request{
		Model: c.Model,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.2,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.HTTPC.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("helper http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", parseErrorEnvelope(resp.StatusCode, body)
	}

	var parsed Response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("helper response: no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func parseErrorEnvelope(status int, body []byte) error {
	var nested errorEnvelopeNested
	if err := json.Unmarshal(body, &nested); err == nil && nested.Error.Message != "" {
		return fmt.Errorf("helper http %d: %s", status, nested.Error.Message)
	}
	var flat errorEnvelopeFlat
	if err := json.Unmarshal(body, &flat); err == nil && flat.Message != "" {
		return fmt.Errorf("helper http %d: %s", status, flat.Message)
	}
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return fmt.Errorf("helper http %d: %s", status, snippet)
}
