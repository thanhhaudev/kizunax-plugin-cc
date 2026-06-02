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
