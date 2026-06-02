package usage

import (
	"context"
	"sync"
)

// FetchAll queries Coding+Credits for every key in parallel. Result slice
// matches input order. Per-key concurrency is capped at maxConcurrentHTTP/2
// (each key fans out to 2 in-flight HTTP requests via Fetch), bounding total
// in-flight HTTP at maxConcurrentHTTP.
//
// One slow key never blocks others; per-key/per-quota errors surface as
// Quota.Err or KeyUsage.AuthFailed.
func (f *Fetcher) FetchAll(ctx context.Context, keys []string) []KeyUsage {
	out := make([]KeyUsage, len(keys))
	keySem := make(chan struct{}, maxConcurrentHTTP/2)

	var wg sync.WaitGroup
	for i, k := range keys {
		i, k := i, k
		wg.Add(1)
		go func() {
			defer wg.Done()
			keySem <- struct{}{}
			defer func() { <-keySem }()
			out[i] = f.Fetch(ctx, k)
		}()
	}
	wg.Wait()
	return out
}
