package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type creditsResponse struct {
	Data struct {
		Total       int64     `json:"total"`
		Consumed    int64     `json:"consumed"`
		Remaining   int64     `json:"remaining"`
		IsUnlimited bool      `json:"is_unlimited"`
		Plan        string    `json:"plan"`
		ResetAt     time.Time `json:"reset_at"`
	} `json:"data"`
}

// fetchCredits GETs /api/v1/quota. Returns:
//   - (quota, false) on 2xx success
//   - (nil, true) on 401/403
//   - (&Quota{Err: ...}, false) on any other error
func (f *Fetcher) fetchCredits(ctx context.Context, apiKey string) (*Quota, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, quotaURL(f.BaseURL), nil)
	if err != nil {
		return &Quota{Kind: "credits", Err: fmt.Sprintf("build request: %v", err)}, false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := f.Client.Do(req)
	if err != nil {
		return &Quota{Kind: "credits", Err: fmt.Sprintf("http: %v", err)}, false
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
		return &Quota{Kind: "credits", Err: fmt.Sprintf("http %d: %s", resp.StatusCode, snippet)}, false
	}

	var parsed creditsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return &Quota{Kind: "credits", Err: fmt.Sprintf("parse: %v", err)}, false
	}
	return &Quota{
		Kind:      "credits",
		Plan:      parsed.Data.Plan,
		Used:      parsed.Data.Consumed,
		Limit:     parsed.Data.Total,
		Remaining: parsed.Data.Remaining,
		ResetAt:   parsed.Data.ResetAt,
		Unlimited: parsed.Data.IsUnlimited,
	}, false
}
