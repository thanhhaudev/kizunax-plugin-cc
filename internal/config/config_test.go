package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestDefaults_HelperHasBaseURLAndModel(t *testing.T) {
	d := Defaults()
	if d.Helper.BaseURL != KizunaXHelperBaseURL {
		t.Fatalf("helper base url: got %q want %q", d.Helper.BaseURL, KizunaXHelperBaseURL)
	}
	if d.Helper.Model != DefaultHelperModel {
		t.Fatalf("helper model: got %q want %q", d.Helper.Model, DefaultHelperModel)
	}
	if d.Helper.TimeoutSeconds != DefaultHelperTimeoutSeconds {
		t.Fatalf("helper timeout: got %d want %d", d.Helper.TimeoutSeconds, DefaultHelperTimeoutSeconds)
	}
}

func TestLoad_HelperKeyReusesProviderPool_WhenHelperKeysEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	mustWriteConfig(t, dir, `{
		"api_keys": ["kx_a", "kx_b"],
		"openai_model": "m",
		"anthropic_model": "m",
		"helper": { "model": "qwen3.5-flash" }
	}`)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HelperAPIKey != "kx_a" && cfg.HelperAPIKey != "kx_b" {
		t.Fatalf("helper key should reuse provider pool, got %q", cfg.HelperAPIKey)
	}
}

func TestLoad_HelperKeyUsesDedicatedPool_WhenHelperKeysSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	mustWriteConfig(t, dir, `{
		"api_keys": ["kx_provider"],
		"openai_model": "m",
		"anthropic_model": "m",
		"helper": { "api_keys": ["kx_helper"] }
	}`)

	cfg, _ := Load("")
	if cfg.HelperAPIKey != "kx_helper" {
		t.Fatalf("helper key should be dedicated kx_helper, got %q", cfg.HelperAPIKey)
	}
	if cfg.APIKey != "kx_provider" {
		t.Fatalf("provider key should stay kx_provider, got %q", cfg.APIKey)
	}
}

func TestLoad_HelperBaseURLEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("KIZUNAX_HELPER_BASE_URL", "http://localhost:1234/v1")
	mustWriteConfig(t, dir, `{"api_keys": ["kx_a"], "anthropic_model": "m"}`)

	cfg, _ := Load("")
	if cfg.Helper.BaseURL != "http://localhost:1234/v1" {
		t.Fatalf("env override ignored: got %q", cfg.Helper.BaseURL)
	}
}

func TestSaveLoad_HelperBlockRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	in := File{
		APIKeys:        []string{"kx_a"},
		AnthropicModel: "m",
		Helper: &HelperConfigFile{
			BaseURL:        "http://example.invalid/v1",
			Model:          "qwen3.5-flash",
			APIKeys:        []string{"kx_helper"},
			TimeoutSeconds: 45,
		},
	}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadFile()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.Helper == nil {
		t.Fatalf("helper block lost")
	}
	if out.Helper.BaseURL != "http://example.invalid/v1" ||
		out.Helper.Model != "qwen3.5-flash" ||
		len(out.Helper.APIKeys) != 1 || out.Helper.APIKeys[0] != "kx_helper" ||
		out.Helper.TimeoutSeconds != 45 {
		t.Fatalf("helper roundtrip mismatch: %+v", out.Helper)
	}
}

func mustWriteConfig(t *testing.T, home, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".kizunax"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".kizunax", "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
