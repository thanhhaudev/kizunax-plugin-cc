package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

type Config struct {
	Provider    string  `json:"provider"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	APIKey      string  `json:"api_key,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
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

func Load() (Config, error) {
	cfg := Defaults()
	path, err := Path()
	if err != nil {
		return cfg, xerrors.Internal("config_path", "cannot resolve home directory", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if envKey := os.Getenv("KIZUNAX_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
			applyEnvOverrides(&cfg)
			return cfg, nil
		}
		return cfg, xerrors.User(
			"config_not_found",
			fmt.Sprintf("config file not found: %s", path),
			"run /kizunax:setup to create it, or set KIZUNAX_API_KEY env var",
		)
	}
	if err != nil {
		return cfg, xerrors.Internal("config_read", "cannot read config file", err)
	}

	if info, statErr := os.Stat(path); statErr == nil {
		mode := info.Mode().Perm()
		if mode&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: %s is world-readable (mode %o). Run: chmod 600 %s\n", path, mode, path)
		}
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg, xerrors.User("config_parse", fmt.Sprintf("invalid JSON in %s: %v", path, err), "")
	}

	merge(&cfg, fileCfg)
	applyEnvOverrides(&cfg)

	if envKey := os.Getenv("KIZUNAX_API_KEY"); envKey != "" {
		cfg.APIKey = envKey
	}

	if cfg.APIKey == "" {
		return cfg, xerrors.User(
			"missing_api_key",
			"no API key configured",
			"add `api_key` to config.json or set KIZUNAX_API_KEY env var",
		)
	}

	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp := path + ".tmp"
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func merge(dst *Config, src Config) {
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.Temperature != 0 {
		dst.Temperature = src.Temperature
	}
	if src.MaxTokens != 0 {
		dst.MaxTokens = src.MaxTokens
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("KIZUNAX_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("KIZUNAX_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("KIZUNAX_MODEL"); v != "" {
		cfg.Model = v
	}
}
