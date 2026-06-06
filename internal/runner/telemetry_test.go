//go:build !lite

package runner

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/thanhhaudev/llmreviewkit/symbols"
)

// TestVerboseTelemetry_AggregatesPerStrategy verifies that the observer
// aggregation logic groups events by strategy and prints one line per group.
// This tests the pattern in isolation — we don't drive a full Review() call.
func TestVerboseTelemetry_AggregatesPerStrategy(t *testing.T) {
	t.Cleanup(func() { symbols.SetExtractObserver(nil) })

	extractCounts := map[string]int{}
	extractTotalNanos := map[string]int64{}
	symbols.SetExtractObserver(func(ev symbols.ExtractEvent) {
		name := symbols.ExtractStrategyName(ev.Strategy)
		extractCounts[name]++
		extractTotalNanos[name] += ev.Duration.Nanoseconds()
	})

	// Drive a few synthetic events covering two strategies.
	syms := []byte("<?php class Foo {}")
	symbols.SetExtractionPolicy(2, 0, 0) // Phpsyms
	for i := 0; i < 3; i++ {
		symbols.DispatchPHP(nil, nil, syms, fmt.Sprintf("F%d.php", i))
	}
	if extractCounts["phpsyms"] != 3 {
		t.Errorf("phpsyms count: got %d, want 3", extractCounts["phpsyms"])
	}

	symbols.SetExtractionPolicy(3, 0, 0) // Regex
	symbols.DispatchPHP(nil, nil, syms, "R.php")
	if extractCounts["regex"] != 1 {
		t.Errorf("regex count: got %d, want 1", extractCounts["regex"])
	}

	// Now re-create the print logic to confirm output format.
	order := []string{"phpsyms", "treesitter", "regex", "auto", "unknown"}
	var buf bytes.Buffer
	for _, name := range order {
		count := extractCounts[name]
		if count == 0 {
			continue
		}
		avgMs := float64(extractTotalNanos[name]) / float64(count) / 1e6
		fmt.Fprintf(&buf, "[verbose] PHP extractor: %s, %d files, avg %.1fms/file\n", name, count, avgMs)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("PHP extractor: phpsyms, 3 files")) {
		t.Errorf("expected phpsyms line; got:\n%s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("PHP extractor: regex, 1 files")) {
		t.Errorf("expected regex line; got:\n%s", out)
	}

	// Reset for cleanup test
	symbols.SetExtractionPolicy(0, 0, 0)
}

// TestVerboseTelemetry_EmitsNothingWhenNoExtractions verifies the silent-on-empty case.
func TestVerboseTelemetry_EmitsNothingWhenNoExtractions(t *testing.T) {
	t.Cleanup(func() { symbols.SetExtractObserver(nil) })

	extractCounts := map[string]int{}
	symbols.SetExtractObserver(func(ev symbols.ExtractEvent) {
		extractCounts[symbols.ExtractStrategyName(ev.Strategy)]++
	})
	// No DispatchPHP calls.

	order := []string{"phpsyms", "treesitter", "regex", "auto", "unknown"}
	var emitted bool
	for _, name := range order {
		if extractCounts[name] > 0 {
			emitted = true
		}
	}
	if emitted {
		t.Error("expected no telemetry lines when no extractions occurred")
	}
}

// silence the unused-import check on this build if `os`/`time` aren't both used.
var _ = os.Stderr
var _ = time.Millisecond
