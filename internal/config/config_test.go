package config

import "testing"

func TestDefaults_ProviderIsAnthropic(t *testing.T) {
	d := Defaults()
	if d.Provider != "anthropic" {
		t.Errorf("Defaults().Provider = %q, want anthropic", d.Provider)
	}
	if d.BaseURL != DefaultAnthropicBaseURL {
		t.Errorf("Defaults().BaseURL = %q, want DefaultAnthropicBaseURL (%s)", d.BaseURL, DefaultAnthropicBaseURL)
	}
	if d.Model != DefaultAnthropicModel {
		t.Errorf("Defaults().Model = %q, want DefaultAnthropicModel (%s)", d.Model, DefaultAnthropicModel)
	}
}

func TestResolveProviderName_FallbackIsAnthropic(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	empty := File{}
	got := resolveProviderName("", empty)
	if got != "anthropic" {
		t.Errorf("resolveProviderName fallback = %q, want anthropic", got)
	}
}

func TestResolveProviderName_ExplicitOpenAIWins(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	f := File{DefaultProvider: "openai"}
	got := resolveProviderName("", f)
	if got != "openai" {
		t.Errorf("resolveProviderName with explicit openai = %q, want openai", got)
	}
}

func TestResolveProviderName_OnlyOpenAIKeys_FallsBackToOpenAIWithNotice(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	noticeOnce.Store(false) // reset so the notice path runs again in this test
	f := File{
		OpenAI:    &ProviderEntry{APIKey: "kx_openai_xxx"},
		Anthropic: nil,
	}
	got := resolveProviderName("", f)
	if got != "openai" {
		t.Errorf("openai-only fallback = %q, want openai", got)
	}
}

func TestResolveProviderName_BothKeys_FallsBackToAnthropic(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	f := File{
		OpenAI:    &ProviderEntry{APIKey: "kx_openai_xxx"},
		Anthropic: &ProviderEntry{APIKey: "kx_anthropic_xxx"},
	}
	got := resolveProviderName("", f)
	if got != "anthropic" {
		t.Errorf("both-keys fallback = %q, want anthropic", got)
	}
}

func TestResolveProviderName_NoKeys_FallsBackToAnthropic(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	f := File{} // no keys at all (truly fresh install)
	got := resolveProviderName("", f)
	if got != "anthropic" {
		t.Errorf("no-keys fallback = %q, want anthropic", got)
	}
}

func TestResolveProviderName_PostMigrateOpenAIOnly_FallsBackToOpenAI(t *testing.T) {
	t.Setenv("KIZUNAX_PROVIDER", "")
	noticeOnce.Store(false)
	// Post-MigrateLegacy shape: single key pool, only OpenAIModel set.
	f := File{
		APIKeys:     []string{"kx_openai_xxx"},
		OpenAIModel: "coding/MiniMax-M2.7",
	}
	got := resolveProviderName("", f)
	if got != "openai" {
		t.Errorf("post-migrate openai-only fallback = %q, want openai", got)
	}
}

func TestModelMaxInputTokens_KnownModels(t *testing.T) {
	cases := map[string]int{
		"coding/MiniMax-M2.7":    114688, // 131072 - 16384
		"coding/kimi-k2.6":       114688,
		"MiniMax-M2.7-highspeed": 114688,
		"MiniMax-M2.5-highspeed": 114688,
	}
	for model, want := range cases {
		if got := ModelMaxInputTokens(model); got != want {
			t.Errorf("ModelMaxInputTokens(%q) = %d, want %d", model, got, want)
		}
	}
}

func TestModelMaxInputTokens_UnknownFallback(t *testing.T) {
	got := ModelMaxInputTokens("some-future-model-3.5")
	if got != 100000 {
		t.Errorf("unknown model = %d, want 100000 fallback", got)
	}
}
