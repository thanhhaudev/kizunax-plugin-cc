package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func dualServer(t *testing.T, codingStatus, creditsStatus int, codingBody, creditsBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/coding/v1/usage":
			w.WriteHeader(codingStatus)
			_, _ = w.Write([]byte(codingBody))
		case "/api/v1/quota":
			w.WriteHeader(creditsStatus)
			_, _ = w.Write([]byte(creditsBody))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestFetch_BothSuccess(t *testing.T) {
	srv := dualServer(t, 200, 200,
		`{"data":{"used":100,"limit":500,"remaining":400,"plan":"pro","resets_at":"2026-06-01T18:30:00Z"}}`,
		`{"data":{"total":100000,"consumed":1000,"remaining":99000,"plan":"pro","reset_at":"2026-06-09T00:00:00Z"}}`,
	)
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	ku := f.Fetch(context.Background(), "kx_TEST1234")
	if ku.AuthFailed {
		t.Errorf("AuthFailed unexpected")
	}
	if ku.Coding == nil || ku.Coding.Used != 100 {
		t.Errorf("Coding not populated: %+v", ku.Coding)
	}
	if ku.Credits == nil || ku.Credits.Used != 1000 {
		t.Errorf("Credits not populated: %+v", ku.Credits)
	}
	if ku.KeyMask != "kx_TEST…" {
		t.Errorf("KeyMask: got %q want %q", ku.KeyMask, "kx_TEST…")
	}
	if ku.KeyHash == "" {
		t.Errorf("KeyHash should be populated")
	}
	if ku.FetchedAt.IsZero() {
		t.Errorf("FetchedAt should be populated")
	}
}

func TestFetch_BothAuth401HoistsAuthFailed(t *testing.T) {
	srv := dualServer(t, 401, 401, `{"error":"x"}`, `{"error":"x"}`)
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	ku := f.Fetch(context.Background(), "kx_BAD")
	if !ku.AuthFailed {
		t.Errorf("expected AuthFailed=true")
	}
	if ku.Coding != nil || ku.Credits != nil {
		t.Errorf("both quotas should be nil on full auth fail")
	}
}

func TestFetch_OnlyCodingAuthFails(t *testing.T) {
	srv := dualServer(t, 401, 200,
		`{"error":"x"}`,
		`{"data":{"total":100000,"consumed":1000,"remaining":99000,"plan":"pro","reset_at":"2026-06-09T00:00:00Z"}}`,
	)
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	ku := f.Fetch(context.Background(), "kx_PARTIAL")
	if ku.AuthFailed {
		t.Errorf("partial auth-fail should NOT hoist to KeyUsage")
	}
	if ku.Coding == nil || ku.Coding.Err == "" {
		t.Errorf("Coding should carry per-quota Err: %+v", ku.Coding)
	}
	if ku.Credits == nil || ku.Credits.Err != "" {
		t.Errorf("Credits should still succeed: %+v", ku.Credits)
	}
}

func TestFetch_OneTimeout(t *testing.T) {
	// Coding returns OK; Credits hangs until client cancels.
	codingOK := `{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/coding/v1/usage" {
			_, _ = w.Write([]byte(codingOK))
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	ku := f.Fetch(ctx, "kx_X")
	if ku.Coding == nil || ku.Coding.Err != "" {
		t.Errorf("coding should succeed")
	}
	if ku.Credits == nil || ku.Credits.Err == "" {
		t.Errorf("credits should carry timeout Err: %+v", ku.Credits)
	}
}

func TestMaskKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"kx_AbCdEfGh", "kx_AbCd…"},
		{"kx_ABCD", "kx_ABCD…"},
		{"kx_A", "kx_A…"},
		{"abcdef", "abcd…"},
		{"abc", "abc…"},
		{"", "(empty)"},
	}
	for _, tc := range cases {
		got := MaskKey(tc.in)
		if got != tc.want {
			t.Errorf("MaskKey(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}
