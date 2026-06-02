package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type codingResponse struct {
	Data struct {
		Used        int64     `json:"used"`
		Limit       int64     `json:"limit"`
		Remaining   int64     `json:"remaining"`
		Plan        string    `json:"plan"`
		ResetsAt    time.Time `json:"resets_at"`
		WindowHours int       `json:"window_hours"`
	} `json:"data"`
}

// fetchCoding GETs /api/coding/v1/usage. Returns:
//   - (quota, false) on 2xx success
//   - (nil, true) on 401/403 (auth-failed signal hoisted by caller)
//   - (&Quota{Err: ...}, false) on any other error
func (f *Fetcher) fetchCoding(ctx context.Context, apiKey string) (*Quota, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL(f.BaseURL), nil)
	if err != nil {
		return &Quota{Kind: "coding", Err: fmt.Sprintf("build request: %v", err)}, false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := f.Client.Do(req)
	if err != nil {
		return &Quota{Kind: "coding", Err: fmt.Sprintf("http: %v", err)}, false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, true
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := string(body)
		if len(snippet) > 120 {
			snippet = snippet[:120]
		}
		return &Quota{Kind: "coding", Err: fmt.Sprintf("http %d: %s", resp.StatusCode, snippet)}, false
	}

	var parsed codingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return &Quota{Kind: "coding", Err: fmt.Sprintf("parse: %v", err)}, false
	}
	return &Quota{
		Kind:      "coding",
		Plan:      parsed.Data.Plan,
		Used:      parsed.Data.Used,
		Limit:     parsed.Data.Limit,
		Remaining: parsed.Data.Remaining,
		ResetAt:   parsed.Data.ResetsAt,
	}, false
}
