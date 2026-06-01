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
	providerOverride := flagValue(args, "--provider")

	if check {
		return setupCheck(providerOverride)
	}
	if rebuild {
		fmt.Println("Use 'bash scripts/build.sh' (or 'make build') to rebuild the binary.")
		return nil
	}

	return setupWizard()
}

func setupCheck(providerOverride string) error {
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
	fmt.Printf("Permissions: %o (%s)\n", info.Mode().Perm(), modeWarning(info.Mode().Perm()))

	file, err := config.LoadFile()
	if err != nil {
		fmt.Printf("Status: ✗ %v\n", err)
		return nil
	}

	// Determine which providers to probe.
	var providers []string
	if providerOverride != "" {
		providers = []string{providerOverride}
	} else {
		providers = configuredProviders(file)
	}

	if len(providers) == 0 {
		fmt.Println("Status: ✗ no providers configured")
		fmt.Println("Run /kizunax:setup to add one.")
		return nil
	}

	fmt.Printf("Default provider: %s\n", config.MigrateLegacy(file).DefaultProvider)
	fmt.Println()

	for _, name := range providers {
		fmt.Printf("[%s]\n", name)
		cfg, err := config.Load(name)
		if err != nil {
			fmt.Printf("  ✗ %v\n", err)
			continue
		}
		fmt.Printf("  base_url: %s\n", cfg.BaseURL)
		fmt.Printf("  model:    %s\n", cfg.Model)
		fmt.Printf("  api_key:  %s\n", maskKey(cfg.APIKey))

		p, err := buildProvider(cfg)
		if err != nil {
			fmt.Printf("  probe:    ✗ %v\n", err)
			continue
		}
		fmt.Print("  probe:    ")
		if err := p.Probe(context.Background()); err != nil {
			fmt.Printf("✗ %v\n", err)
		} else {
			fmt.Println("✓ ok")
		}
		fmt.Println()
	}
	return nil
}

func setupWizard() error {
	fmt.Println("Kizunax interactive setup")
	fmt.Println("(Press Enter to accept defaults in [brackets].)")
	fmt.Println()

	file, _ := config.LoadFile()
	hadExisting := file.OpenAI != nil || file.Anthropic != nil || file.Provider != ""
	file = config.MigrateLegacy(file)

	reader := bufio.NewReader(os.Stdin)

	if hadExisting {
		if !askYesNo(reader, "Existing config found. Reconfigure?", false) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// --- OpenAI ---
	if askYesNo(reader, "Configure openai provider?", true) {
		entry := config.ProviderEntry{}
		if file.OpenAI != nil {
			entry = *file.OpenAI
		}
		if entry.BaseURL == "" {
			entry.BaseURL = config.DefaultOpenAIBaseURL
		}
		if entry.Model == "" {
			entry.Model = config.DefaultOpenAIModel
		}
		entry.BaseURL = ask(reader, "  Base URL", entry.BaseURL)
		entry.Model = ask(reader, "  Model", entry.Model)
		key := promptKey(reader, "  API key", entry.APIKey)
		if key == "" {
			return xerrors.User("empty_key", "API key required for openai", "")
		}
		entry.APIKey = key
		file.OpenAI = &entry
	} else {
		file.OpenAI = nil
	}

	// --- Anthropic ---
	if askYesNo(reader, "Configure anthropic provider?", true) {
		entry := config.ProviderEntry{}
		if file.Anthropic != nil {
			entry = *file.Anthropic
		}
		if entry.BaseURL == "" {
			entry.BaseURL = config.DefaultAnthropicBaseURL
		}
		if entry.Model == "" {
			entry.Model = config.DefaultAnthropicModel
		}
		entry.BaseURL = ask(reader, "  Base URL", entry.BaseURL)
		entry.Model = ask(reader, "  Model", entry.Model)

		var key string
		if file.OpenAI != nil && file.OpenAI.APIKey != "" {
			if askYesNo(reader, "  Use same API key as openai?", true) {
				key = file.OpenAI.APIKey
			}
		}
		if key == "" {
			key = promptKey(reader, "  API key", entry.APIKey)
		}
		if key == "" {
			return xerrors.User("empty_key", "API key required for anthropic", "")
		}
		entry.APIKey = key
		file.Anthropic = &entry
	} else {
		file.Anthropic = nil
	}

	// --- Default provider ---
	switch {
	case file.OpenAI != nil && file.Anthropic != nil:
		def := file.DefaultProvider
		if def != "openai" && def != "anthropic" {
			def = "openai"
		}
		choice := ask(reader, "Default provider [openai|anthropic]", def)
		if choice != "openai" && choice != "anthropic" {
			return xerrors.User("bad_default",
				fmt.Sprintf("invalid default provider %q", choice),
				"choose openai or anthropic")
		}
		file.DefaultProvider = choice
	case file.OpenAI != nil:
		file.DefaultProvider = "openai"
	case file.Anthropic != nil:
		file.DefaultProvider = "anthropic"
	default:
		return xerrors.User("no_provider", "no provider configured", "")
	}

	if err := config.Save(file); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	path, _ := config.Path()
	fmt.Println()
	fmt.Printf("✓ Saved %s (mode 0600)\n", path)
	fmt.Printf("Default provider: %s\n", file.DefaultProvider)
	if file.OpenAI != nil {
		fmt.Printf("  openai:    model=%s, key=%s\n", file.OpenAI.Model, maskKey(file.OpenAI.APIKey))
	}
	if file.Anthropic != nil {
		fmt.Printf("  anthropic: model=%s, key=%s\n", file.Anthropic.Model, maskKey(file.Anthropic.APIKey))
	}
	fmt.Println()
	fmt.Println("Next: try /kizunax:review on a repo with uncommitted changes.")
	fmt.Println("Switch provider on the fly: kizunax review --provider anthropic")
	return nil
}

func configuredProviders(file config.File) []string {
	migrated := config.MigrateLegacy(file)
	var out []string
	if migrated.OpenAI != nil {
		out = append(out, "openai")
	}
	if migrated.Anthropic != nil {
		out = append(out, "anthropic")
	}
	return out
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

func askYesNo(r *bufio.Reader, label string, defaultYes bool) bool {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Printf("%s %s: ", label, suffix)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

func promptKey(r *bufio.Reader, label, existing string) string {
	suffix := ""
	if existing != "" {
		suffix = " (Enter to keep existing)"
	}
	fmt.Printf("%s%s: ", label, suffix)
	line, _ := r.ReadString('\n')
	key := strings.TrimSpace(line)
	if key == "" && existing != "" {
		return existing
	}
	return key
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
		"supported: openai, anthropic")
}
