package usage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchAll_OrderPreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		if r.URL.Path == "/api/coding/v1/usage" {
			fmt.Fprintf(w, `{"data":{"used":%d,"limit":100,"remaining":%d,"plan":"pro","resets_at":"2026-06-01T18:30:00Z"}}`, len(key), 100-len(key))
		} else {
			fmt.Fprintf(w, `{"data":{"total":1000,"consumed":%d,"remaining":%d,"plan":"pro","reset_at":"2026-06-09T00:00:00Z"}}`, len(key), 1000-len(key))
		}
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}

	keys := []string{"kx_A", "kx_BB", "kx_CCC"}
	usages := f.FetchAll(context.Background(), keys)
	if len(usages) != 3 {
		t.Fatalf("len: got %d want 3", len(usages))
	}
	for i, ku := range usages {
		expectedLen := int64(len("Bearer kx_") + i + 1) // grows by 1 per key
		if ku.Coding.Used != expectedLen {
			t.Errorf("usage[%d] order mismatch: Used=%d expected %d", i, ku.Coding.Used, expectedLen)
		}
	}
}

func TestFetchAll_OneSlowDoesNotBlockOthers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer kx_SLOW" {
			time.Sleep(80 * time.Millisecond)
		}
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","total":10,"consumed":1,"reset_at":"2026-06-09T00:00:00Z"}}`))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}

	start := time.Now()
	usages := f.FetchAll(context.Background(), []string{"kx_FAST", "kx_SLOW", "kx_FAST2"})
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Errorf("slow key blocked fast ones: %v", elapsed)
	}
	if len(usages) != 3 {
		t.Fatalf("len: got %d", len(usages))
	}
}

func TestFetchAll_SemaphoreCapsConcurrency(t *testing.T) {
	var inflight, peak int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt64(&inflight, 1)
		defer atomic.AddInt64(&inflight, -1)
		for {
			p := atomic.LoadInt64(&peak)
			if cur > p {
				if atomic.CompareAndSwapInt64(&peak, p, cur) {
					break
				}
				continue
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"data":{"used":1,"limit":10,"remaining":9,"plan":"free","resets_at":"2026-06-01T18:30:00Z","total":10,"consumed":1,"reset_at":"2026-06-09T00:00:00Z"}}`))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), BaseURL: srv.URL}

	// 50 keys × 2 endpoints = 100 candidate requests; semaphore is maxConcurrentHTTP/2 = 16 keys → up to 32 HTTP in-flight.
	keys := make([]string, 50)
	for i := range keys {
		keys[i] = fmt.Sprintf("kx_K%d", i)
	}
	_ = f.FetchAll(context.Background(), keys)

	if peak > int64(maxConcurrentHTTP) {
		t.Errorf("peak concurrency %d exceeded cap %d", peak, maxConcurrentHTTP)
	}
	if peak < 5 {
		t.Errorf("peak concurrency suspiciously low (%d) — semaphore may be too aggressive", peak)
	}
}
