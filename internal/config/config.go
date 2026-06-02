package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

// ProviderEntry stores per-provider runtime fields.
type ProviderEntry struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
}

// File is the on-disk layout. v0.6.6+ format:
//
//	{ "api_keys": [...], "rotation": "round-robin",
//	  "openai_model": "...", "anthropic_model": "..." }
//
// Legacy v0.6.5 fields below are READ ONLY for one-way migration via
// MigrateLegacy. Save() never writes them (all have omitempty).
type File struct {
	APIKeys        []string `json:"api_keys,omitempty"`
	Rotation       string   `json:"rotation,omitempty"`
	OpenAIModel    string   `json:"openai_model,omitempty"`
	AnthropicModel string   `json:"anthropic_model,omitempty"`

	// Legacy v0.6.5 multi-provider format.
	DefaultProvider string         `json:"default_provider,omitempty"`
	OpenAI          *ProviderEntry `json:"openai,omitempty"`
	Anthropic       *ProviderEntry `json:"anthropic,omitempty"`

	// Legacy v0.5 flat format.
	Provider string `json:"provider,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Model    string `json:"model,omitempty"`
	APIKey   string `json:"api_key,omitempty"`

	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// Config is the runtime-resolved single-provider effective config.
type Config struct {
	Provider    string
	BaseURL     string
	Model       string
	APIKey      string
	Temperature float64
	MaxTokens   int
}

func Defaults() Config {
	return Config{
		Provider:    "anthropic",
		BaseURL:     DefaultAnthropicBaseURL,
		Model:       DefaultAnthropicModel,
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
	}
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kizunax", "config.json"), nil
}

// LoadFile reads the raw config file. Returns zero-File if missing.
func LoadFile() (File, error) {
	var f File
	path, err := Path()
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, xerrors.User("config_parse",
			fmt.Sprintf("invalid JSON in %s: %v", path, err), "")
	}
	return f, nil
}

// keyCounter drives round-robin key selection across config.Load calls.
var keyCounter atomic.Uint64

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func pickKey(file File) string {
	n := uint64(len(file.APIKeys))
	if n == 0 {
		return ""
	}
	i := keyCounter.Add(1) - 1
	return file.APIKeys[i%n]
}

// Load resolves the effective config. providerOverride from --provider flag
// takes highest precedence, then env, then file default (anthropic since
// v0.10; explicit "openai" in default_provider still honored). Each call
// rotates to the next API key in the configured pool.
func Load(providerOverride string) (Config, error) {
	cfg := Defaults()

	path, err := Path()
	if err != nil {
		return cfg, xerrors.Internal("config_path", "cannot resolve home directory", err)
	}

	file, fileErr := LoadFile()
	if fileErr != nil {
		return cfg, fileErr
	}
	file = MigrateLegacy(file)

	if info, statErr := os.Stat(path); statErr == nil {
		if info.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: %s is world-readable (mode %o). Run: chmod 600 %s\n",
				path, info.Mode().Perm(), path)
		}
	}

	provider := resolveProviderName(providerOverride, file)
	cfg.Provider = provider

	switch provider {
	case "anthropic":
		cfg.BaseURL = KizunaXAnthropicBaseURL
		cfg.Model = firstNonEmpty(file.AnthropicModel, DefaultAnthropicModel)
	default:
		cfg.BaseURL = KizunaXOpenAIBaseURL
		cfg.Model = firstNonEmpty(file.OpenAIModel, DefaultOpenAIModel)
	}

	cfg.APIKey = pickKey(file)

	if file.Temperature != 0 {
		cfg.Temperature = file.Temperature
	}
	if file.MaxTokens != 0 {
		cfg.MaxTokens = file.MaxTokens
	}

	if v := os.Getenv("KIZUNAX_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("KIZUNAX_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("KIZUNAX_API_KEY"); v != "" {
		cfg.APIKey = v
	}

	if cfg.APIKey == "" {
		return cfg, xerrors.User(
			"missing_api_key",
			fmt.Sprintf("no API key for provider %q", provider),
			"run /kizunax:setup or set KIZUNAX_API_KEY env var",
		)
	}

	return cfg, nil
}

// Save writes File in the v0.6.6 schema. Legacy fields are intentionally
// blanked so old keys do not linger on disk.
func Save(f File) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Blank legacy fields before marshal so they're dropped via omitempty.
	f.DefaultProvider = ""
	f.OpenAI = nil
	f.Anthropic = nil
	f.Provider = ""
	f.BaseURL = ""
	f.Model = ""
	f.APIKey = ""
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MigrateLegacy converts a v0.6.5 multi-provider or older flat file into the
// v0.6.6 single-pool schema in memory. No-op if APIKeys is already populated.
func MigrateLegacy(f File) File {
	if len(f.APIKeys) > 0 {
		if f.Rotation == "" {
			f.Rotation = RotationRoundRobin
		}
		return f
	}

	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	if f.OpenAI != nil {
		add(f.OpenAI.APIKey)
	}
	if f.Anthropic != nil {
		add(f.Anthropic.APIKey)
	}
	add(f.APIKey) // legacy flat
	f.APIKeys = keys

	if f.OpenAIModel == "" {
		switch {
		case f.OpenAI != nil && f.OpenAI.Model != "":
			f.OpenAIModel = f.OpenAI.Model
		case f.Provider == "openai" && f.Model != "":
			f.OpenAIModel = f.Model
		}
	}
	if f.AnthropicModel == "" {
		switch {
		case f.Anthropic != nil && f.Anthropic.Model != "":
			f.AnthropicModel = f.Anthropic.Model
		case f.Provider == "anthropic" && f.Model != "":
			f.AnthropicModel = f.Model
		}
	}
	if f.Rotation == "" {
		f.Rotation = RotationRoundRobin
	}
	return f
}

func resolveProviderName(override string, file File) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("KIZUNAX_PROVIDER"); v != "" {
		return v
	}
	if file.DefaultProvider == "openai" || file.DefaultProvider == "anthropic" {
		return file.DefaultProvider
	}
	return "anthropic"
}

// modelInputBudget is the per-model context-window minus a generous output
// reserve. Source: KizunaX Coding Plan probe 2026-06-01 confirmed
// context_window=131072 and max_output_tokens=16384 for MiniMax-M2.x and
// Kimi-K2.6. Subtract output reserve → 114688 input budget. Anthropic-shape
// model IDs reuse the same backend so the budget is identical.
var modelInputBudget = map[string]int{
	"coding/MiniMax-M2.7":    114688,
	"coding/MiniMax-M2.5":    114688,
	"coding/kimi-k2.6":       114688,
	"MiniMax-M2.7-highspeed": 114688,
	"MiniMax-M2.5-highspeed": 114688,
}

// ModelMaxInputTokens returns the input-token budget for a model. Unknown
// models fall back to a conservative 100000 (smaller than any current
// KizunaX-served model) so an oversize prompt fails fast rather than
// silently exceeding the real cap.
func ModelMaxInputTokens(model string) int {
	if v, ok := modelInputBudget[model]; ok {
		return v
	}
	return 100000
}
