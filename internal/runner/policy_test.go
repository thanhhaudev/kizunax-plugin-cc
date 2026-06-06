package runner

import (
	"testing"

	"github.com/thanhhaudev/llmreviewkit/engine"
)

func TestResolveExtractionPolicy_UnsetIsAuto(t *testing.T) {
	t.Setenv("KIZUNAX_PHP_EXTRACTOR", "")
	p := resolveExtractionPolicy()
	if p.PHP != engine.StrategyAuto {
		t.Errorf("unset env: got %v, want StrategyAuto", p.PHP)
	}
	// Defaults from engine.DefaultExtractionPolicy should be propagated.
	if p.ExtractionTimeout == 0 {
		t.Error("expected non-zero default ExtractionTimeout")
	}
	if p.MaxFileSize == 0 {
		t.Error("expected non-zero default MaxFileSize")
	}
}

func TestResolveExtractionPolicy_KnownValues(t *testing.T) {
	cases := map[string]engine.ExtractionStrategy{
		"auto":        engine.StrategyAuto,
		"AUTO":        engine.StrategyAuto,
		"gonative":    engine.StrategyGoNative,
		"GoNative":    engine.StrategyGoNative,
		"treesitter":  engine.StrategyTreeSitter,
		"regex":       engine.StrategyRegex,
		"  gonative ": engine.StrategyGoNative, // trim
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Setenv("KIZUNAX_PHP_EXTRACTOR", in)
			got := resolveExtractionPolicy().PHP
			if got != want {
				t.Errorf("env=%q: got %v, want %v", in, got, want)
			}
		})
	}
}

func TestResolveExtractionPolicy_UnknownIsAuto(t *testing.T) {
	t.Setenv("KIZUNAX_PHP_EXTRACTOR", "bogus")
	if got := resolveExtractionPolicy().PHP; got != engine.StrategyAuto {
		t.Errorf("unknown env: got %v, want StrategyAuto (fallback)", got)
	}
}
