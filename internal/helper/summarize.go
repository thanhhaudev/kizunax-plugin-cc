package helper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/schema"
)

const (
	maxSummarizeInputBytes = 2 * 1024
	summarizeMaxTokens     = 300
)

const summarizeSystemPrompt = "You are a concise reviewer. Summarize the findings below in 2-3 sentences for a busy reviewer. Lead with the verdict and finding count, then call out the single most critical issue. Plain prose only — no markdown, no lists."

// Summarize calls the Public v1 helper with a compact serialization of the
// review result and returns a short TL;DR string. Empty findings → ("", nil).
// Any helper failure is returned as an error; callers (runner) decide
// whether to log + continue or surface.
func Summarize(ctx context.Context, cfg config.HelperConfig, apiKey string, result schema.ReviewResult) (string, error) {
	if len(result.Findings) == 0 {
		return "", nil
	}

	user := serializeFindings(result)

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &Client{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		APIKey:  apiKey,
		HTTPC:   &http.Client{Timeout: timeout},
	}
	return client.Chat(ctx, summarizeSystemPrompt, user, summarizeMaxTokens)
}

func serializeFindings(r schema.ReviewResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Verdict: %s\nFindings (%d):\n", r.Verdict, len(r.Findings))
	for i, f := range r.Findings {
		fmt.Fprintf(&sb, "%d. [%s] %s:%d-%d — %s\n",
			i+1, f.Severity, f.File, f.LineStart, f.LineEnd, f.Title)
		body := strings.TrimSpace(f.Body)
		if body != "" {
			if len(body) > 240 {
				body = truncateUTF8(body, 240) + "…"
			}
			fmt.Fprintf(&sb, "   %s\n", body)
		}
		if sb.Len() > maxSummarizeInputBytes {
			break
		}
	}
	out := sb.String()
	if len(out) > maxSummarizeInputBytes {
		out = truncateUTF8(out, maxSummarizeInputBytes)
	}
	return out
}

// truncateUTF8 returns s shortened to at most maxBytes, snapped back to the
// nearest rune boundary so the result is always valid UTF-8.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return ""
}
