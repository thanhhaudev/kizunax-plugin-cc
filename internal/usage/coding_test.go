package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCoding_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/coding/v1/usage" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer kx_test" {
			t.Errorf("missing/wrong auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"used":12500,"limit":50000,"remaining":37500,"plan":"pro","resets_at":"2026-06-01T18:30:00Z","window_hours":5}}`))
	}))
	defer srv.Close()

	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCoding(context.Background(), "kx_test")
	if authFail {
		t.Fatalf("authFail unexpected")
	}
	if q == nil || q.Err != "" {
		t.Fatalf("quota err: %+v", q)
	}
	if q.Kind != "coding" || q.Plan != "pro" || q.Used != 12500 || q.Limit != 50000 || q.Remaining != 37500 {
		t.Errorf("fields not parsed: %+v", q)
	}
	if q.ResetAt.IsZero() {
		t.Errorf("ResetAt should be parsed")
	}
}

func TestFetchCoding_Auth401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCoding(context.Background(), "kx_bad")
	if !authFail {
		t.Errorf("expected authFail=true")
	}
	if q != nil {
		t.Errorf("expected nil quota on auth fail")
	}
}

func TestFetchCoding_Server503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down for maintenance", 503)
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCoding(context.Background(), "kx_test")
	if authFail {
		t.Errorf("503 is not auth fail")
	}
	if q == nil || q.Err == "" {
		t.Fatalf("expected Err populated, got %+v", q)
	}
}

func TestFetchCoding_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, authFail := f.fetchCoding(context.Background(), "kx_test")
	if authFail {
		t.Errorf("parse err is not auth fail")
	}
	if q == nil || q.Err == "" {
		t.Fatalf("expected Err populated")
	}
}

func TestFetchCoding_TolerateExtraFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","window_hours":5,"some_future_field":"value"}}`))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}
	q, _ := f.fetchCoding(context.Background(), "kx_test")
	if q == nil || q.Err != "" {
		t.Errorf("extra fields should not break parse: %+v", q)
	}
}
