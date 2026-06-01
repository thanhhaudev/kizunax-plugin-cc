package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

// ProviderEntry stores per-provider runtime fields.
type ProviderEntry struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
}

// File is the on-disk layout. Supports BOTH:
//   1. New multi-provider format: openai{} + anthropic{} blocks + default_provider
//   2. Legacy single-provider format: flat provider/base_url/model/api_key
// Load() auto-detects and resolves.
type File struct {
	DefaultProvider string         `json:"default_provider,omitempty"`
	OpenAI          *ProviderEntry `json:"openai,omitempty"`
	Anthropic       *ProviderEntry `json:"anthropic,omitempty"`

	// Legacy fields — kept for backward compat reading.
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
		Provider:    "openai",
		BaseURL:     DefaultOpenAIBaseURL,
		Model:       DefaultOpenAIModel,
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

// Load resolves the effective config. providerOverride from --provider flag
// takes highest precedence, then env, then file default, then "openai".
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

	// Warn if file mode is too permissive.
	if info, statErr := os.Stat(path); statErr == nil {
		if info.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: %s is world-readable (mode %o). Run: chmod 600 %s\n",
				path, info.Mode().Perm(), path)
		}
	}

	// Determine which provider to use.
	provider := resolveProviderName(providerOverride, file)
	cfg.Provider = provider

	// Look up provider entry (new format first, then legacy fallback).
	entry := lookupProvider(file, provider)
	if entry != nil {
		cfg.BaseURL = entry.BaseURL
		cfg.Model = entry.Model
		cfg.APIKey = entry.APIKey
	}

	// Apply defaults for empty fields based on chosen provider.
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURLFor(provider)
	}
	if cfg.Model == "" {
		cfg.Model = defaultModelFor(provider)
	}

	if file.Temperature != 0 {
		cfg.Temperature = file.Temperature
	}
	if file.MaxTokens != 0 {
		cfg.MaxTokens = file.MaxTokens
	}

	// Env overrides (after file, before final validation).
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

// Save writes File in the new multi-provider format.
func Save(f File) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
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

// MigrateLegacy converts a flat legacy file to the new multi-provider format
// in memory. No-op if already in new format.
func MigrateLegacy(f File) File {
	if f.OpenAI != nil || f.Anthropic != nil {
		return f
	}
	if f.Provider == "" && f.BaseURL == "" && f.Model == "" && f.APIKey == "" {
		return f
	}
	entry := &ProviderEntry{
		BaseURL: f.BaseURL,
		Model:   f.Model,
		APIKey:  f.APIKey,
	}
	switch f.Provider {
	case "anthropic":
		f.Anthropic = entry
	default:
		f.OpenAI = entry
	}
	if f.DefaultProvider == "" {
		if f.Provider != "" {
			f.DefaultProvider = f.Provider
		} else {
			f.DefaultProvider = "openai"
		}
	}
	// Clear legacy
	f.Provider = ""
	f.BaseURL = ""
	f.Model = ""
	f.APIKey = ""
	return f
}

func resolveProviderName(override string, file File) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("KIZUNAX_PROVIDER"); v != "" {
		return v
	}
	if file.DefaultProvider != "" {
		return file.DefaultProvider
	}
	// New format file but no default? Pick first configured.
	if file.OpenAI != nil {
		return "openai"
	}
	if file.Anthropic != nil {
		return "anthropic"
	}
	// Legacy
	if file.Provider != "" {
		return file.Provider
	}
	return "openai"
}

func lookupProvider(file File, provider string) *ProviderEntry {
	switch provider {
	case "openai":
		if file.OpenAI != nil {
			return file.OpenAI
		}
		// Legacy fallback: only if legacy file's provider matches (or is empty).
		if file.Provider == "openai" || file.Provider == "" {
			if file.APIKey != "" || file.BaseURL != "" || file.Model != "" {
				return &ProviderEntry{
					BaseURL: file.BaseURL,
					Model:   file.Model,
					APIKey:  file.APIKey,
				}
			}
		}
	case "anthropic":
		if file.Anthropic != nil {
			return file.Anthropic
		}
		if file.Provider == "anthropic" {
			return &ProviderEntry{
				BaseURL: file.BaseURL,
				Model:   file.Model,
				APIKey:  file.APIKey,
			}
		}
	}
	return nil
}

func defaultBaseURLFor(provider string) string {
	if provider == "anthropic" {
		return DefaultAnthropicBaseURL
	}
	return DefaultOpenAIBaseURL
}

func defaultModelFor(provider string) string {
	if provider == "anthropic" {
		return DefaultAnthropicModel
	}
	return DefaultOpenAIModel
}
