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
	if pa, ok := p.(*provider.OpenAIAdapter); ok {
		if err := pa.Probe(context.Background()); err != nil {
			fmt.Printf("✗ %v\n", err)
			return nil
		}
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

	cfg.BaseURL = prompt(reader, "Base URL", cfg.BaseURL)
	cfg.Model = prompt(reader, "Model", cfg.Model)

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

func prompt(r *bufio.Reader, label, def string) string {
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
	}
	return nil, xerrors.User("unknown_provider",
		fmt.Sprintf("provider %q is not supported in v0.1", cfg.Provider),
		"v0.1 supports only OpenAI-compatible providers")
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}
