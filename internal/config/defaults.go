package config

import "time"

const (
	DefaultOpenAIBaseURL    = "https://kizunax.io/api/coding/v1"
	DefaultAnthropicBaseURL = "https://kizunax.io/api/coding/anthropic/v1"

	// KizunaX endpoint constants — preferred over the Default* names at call
	// sites that always target KizunaX (e.g. config.Load and the setup-web
	// /list-models handler). The two pairs are intentionally equal strings;
	// the alias makes intent obvious.
	KizunaXOpenAIBaseURL    = DefaultOpenAIBaseURL
	KizunaXAnthropicBaseURL = DefaultAnthropicBaseURL

	DefaultOpenAIModel    = "coding/MiniMax-M2.7"
	DefaultAnthropicModel = "MiniMax-M2.7-highspeed"

	AnthropicVersion = "2023-06-01"

	DefaultTemperature = 0.1
	DefaultMaxTokens   = 8192

	MaxDiffBytes    = 256 * 1024
	MaxOutputTokens = 16384

	HTTPTimeout = 120 * time.Second

	RotationRoundRobin = "round-robin"

	KizunaXHelperBaseURL        = "https://kizunax.io/api/v1"
	DefaultHelperModel          = "qwen3.5-flash"
	DefaultHelperTimeoutSeconds = 30
)
