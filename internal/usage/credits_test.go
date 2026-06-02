package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCredits_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/quota" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"total":100000,"consumed":1234,"remaining":98766,"is_unlimited":false,"cycle_consumed":1234,"period_start":"2026-05-09T08:47:38.99605+00:00","period_end":"2026-06-09T08:47:38.99605+00:00","plan":"free","reset_at":"2026-06-09T08:47:38.99605+00:00"}}`))
	}))
	defer srv.Close()

	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCredits(context.Background(), "kx_test")
	if authFail {
		t.Fatalf("authFail unexpected")
	}
	if q == nil || q.Err != "" {
		t.Fatalf("quota err: %+v", q)
	}
	if q.Kind != "credits" || q.Used != 1234 || q.Limit != 100000 || q.Remaining != 98766 || q.Plan != "free" {
		t.Errorf("fields not parsed: %+v", q)
	}
	if q.Unlimited {
		t.Errorf("Unlimited should be false")
	}
}

func TestFetchCredits_Unlimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"total":0,"consumed":5000,"remaining":0,"is_unlimited":true,"plan":"enterprise","reset_at":"2026-06-09T08:47:38.99605+00:00"}}`))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, _ := f.fetchCredits(context.Background(), "kx_test")
	if q == nil || !q.Unlimited {
		t.Fatalf("Unlimited not propagated: %+v", q)
	}
}

func TestFetchCredits_Auth401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCredits(context.Background(), "kx_bad")
	if !authFail || q != nil {
		t.Errorf("expected authFail=true, nil quota")
	}
}

func TestFetchCredits_Server500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, _ := f.fetchCredits(context.Background(), "kx_test")
	if q == nil || q.Err == "" {
		t.Errorf("expected Err populated, got %+v", q)
	}
}
