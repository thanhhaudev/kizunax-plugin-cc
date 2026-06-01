package config

import "time"

const (
	DefaultOpenAIBaseURL    = "https://kizunax.io/api/coding/v1"
	DefaultAnthropicBaseURL = "https://kizunax.io/api/coding/anthropic/v1"

	DefaultOpenAIModel    = "coding/MiniMax-M2.7"
	DefaultAnthropicModel = "MiniMax-M2.7-highspeed"

	AnthropicVersion = "2023-06-01"

	DefaultTemperature = 0.1
	DefaultMaxTokens   = 8192

	MaxDiffBytes    = 256 * 1024
	MaxOutputTokens = 16384

	HTTPTimeout = 120 * time.Second
)
