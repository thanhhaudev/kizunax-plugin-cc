package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
)

func runSetup(args []string) error {
	check := hasFlag(args, "--check")
	rebuild := hasFlag(args, "--rebuild")

	if check {
		return setupCheck()
	}
	if rebuild {
		fmt.Println("Use 'bash scripts/build.sh' (or 'make build') to rebuild the binary.")
		return nil
	}

	return setupWizard()
}

func setupCheck() error {
	fmt.Println("Kizunax setup check")
	fmt.Println()

	path, _ := config.Path()
	fmt.Printf("Config path: %s\n", path)

	info, err := os.Stat(path)
	if err != nil {
		fmt.Println("Status: ✗ config file not found")
		fmt.Println("Run /kizunax:setup to create it.")
		return nil
	}
	mode := info.Mode().Perm()
	fmt.Printf("Permissions: %o (%s)\n", mode, modeWarning(mode))

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Status: ✗ %v\n", err)
		return nil
	}
	fmt.Printf("Provider: %s\n", cfg.Provider)
	fmt.Printf("Base URL: %s\n", cfg.BaseURL)
	fmt.Printf("Model:    %s\n", cfg.Model)
	fmt.Printf("API key:  %s\n", maskKey(cfg.APIKey))
	fmt.Println()

	fmt.Print("Probe provider (1 tiny request)... ")
	p, err := buildProvider(cfg)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		return nil
	}
	if err := p.Probe(context.Background()); err != nil {
		fmt.Printf("✗ %v\n", err)
		return nil
	}
	fmt.Println("✓ ok")
	return nil
}

func setupWizard() error {
	fmt.Println("Kizunax interactive setup")
	fmt.Println("(Press Enter to accept defaults shown in [brackets].)")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	cfg := config.Defaults()

	if existing, err := config.Load(); err == nil {
		fmt.Print("Existing config found. Reconfigure? [y/N]: ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(resp)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
		cfg = existing
		cfg.APIKey = "" // force re-enter
	}

	cfg.Provider = ask(reader, "Provider [openai|anthropic]", cfg.Provider)
	switch cfg.Provider {
	case "anthropic":
		if cfg.BaseURL == config.DefaultOpenAIBaseURL || cfg.BaseURL == "" {
			cfg.BaseURL = config.DefaultAnthropicBaseURL
		}
		if cfg.Model == config.DefaultOpenAIModel || cfg.Model == "" {
			cfg.Model = config.DefaultAnthropicModel
		}
	case "openai", "":
		cfg.Provider = "openai"
		if cfg.BaseURL == "" {
			cfg.BaseURL = config.DefaultOpenAIBaseURL
		}
		if cfg.Model == "" {
			cfg.Model = config.DefaultOpenAIModel
		}
	default:
		return xerrors.User("bad_provider",
			fmt.Sprintf("unknown provider %q", cfg.Provider),
			"supported: openai, anthropic")
	}
	cfg.BaseURL = ask(reader, "Base URL", cfg.BaseURL)
	cfg.Model = ask(reader, "Model", cfg.Model)

	fmt.Print("API key (input shown): ")
	key, _ := reader.ReadString('\n')
	cfg.APIKey = strings.TrimSpace(key)
	if cfg.APIKey == "" {
		return xerrors.User("empty_key", "API key is required", "")
	}

	if err := config.Save(cfg); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	path, _ := config.Path()
	fmt.Println()
	fmt.Printf("✓ Saved %s (mode 0600)\n", path)
	fmt.Println()
	fmt.Println("Next: try /kizunax:review on a repo with uncommitted changes.")
	return nil
}

func ask(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func maskKey(k string) string {
	if len(k) < 8 {
		return "(invalid)"
	}
	return k[:6] + "..." + k[len(k)-4:]
}

func modeWarning(mode os.FileMode) string {
	if mode&0o077 != 0 {
		return "⚠ world-readable; run chmod 600"
	}
	return "ok"
}

func buildProvider(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "openai", "":
		return provider.NewOpenAI(cfg), nil
	case "anthropic":
		return provider.NewAnthropic(cfg), nil
	}
	return nil, xerrors.User("unknown_provider",
		fmt.Sprintf("provider %q is not supported", cfg.Provider),
		"supported: openai (default), anthropic")
}

