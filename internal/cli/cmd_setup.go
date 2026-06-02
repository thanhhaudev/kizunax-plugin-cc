package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/provider"
)

func runSetup(args []string) error {
	check := hasFlag(args, "--check")
	rebuild := hasFlag(args, "--rebuild")
	jsonOut := hasFlag(args, "--json")
	save := hasFlag(args, "--save")
	apply := hasFlag(args, "--apply")
	clearPending := hasFlag(args, "--clear-pending")
	web := hasFlag(args, "--web")
	providerOverride := flagValue(args, "--provider")

	if check {
		return setupCheck(providerOverride)
	}
	if rebuild {
		fmt.Println("Use 'bash scripts/build.sh' (or 'make build') to rebuild the binary.")
		return nil
	}
	if jsonOut {
		return setupJSON()
	}
	if save {
		return setupSave(args)
	}
	if apply {
		return setupApply(args)
	}
	if clearPending {
		return setupClearPending()
	}
	if web {
		return setupWeb()
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

// setupJSON prints the current config inventory as JSON. No prompts.
// has_api_key is the only field that exposes secret state — the key itself is never printed.
func setupJSON() error {
	path, _ := config.Path()

	type providerSummary struct {
		BaseURL   string `json:"base_url"`
		Model     string `json:"model"`
		HasAPIKey bool   `json:"has_api_key"`
	}
	out := struct {
		ConfigPath      string                     `json:"config_path"`
		DefaultProvider string                     `json:"default_provider"`
		Providers       map[string]providerSummary `json:"providers"`
	}{
		ConfigPath: path,
		Providers:  map[string]providerSummary{},
	}

	file, err := config.LoadFile()
	if err == nil {
		file = config.MigrateLegacy(file)
		out.DefaultProvider = file.DefaultProvider
		if file.OpenAI != nil {
			out.Providers["openai"] = providerSummary{
				BaseURL:   file.OpenAI.BaseURL,
				Model:     file.OpenAI.Model,
				HasAPIKey: file.OpenAI.APIKey != "",
			}
		}
		if file.Anthropic != nil {
			out.Providers["anthropic"] = providerSummary{
				BaseURL:   file.Anthropic.BaseURL,
				Model:     file.Anthropic.Model,
				HasAPIKey: file.Anthropic.APIKey != "",
			}
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// setupSave writes a provider entry into config.json without prompting.
// Required flags: --provider, --base-url, --model, --api-key
// Optional: --set-default, --reuse-api-key
func setupSave(args []string) error {
	provider := flagValue(args, "--provider")
	baseURL := flagValue(args, "--base-url")
	model := flagValue(args, "--model")
	apiKey := flagValue(args, "--api-key")
	setDefault := hasFlag(args, "--set-default")
	reuseKey := hasFlag(args, "--reuse-api-key")

	if provider != "openai" && provider != "anthropic" {
		return xerrors.User("bad_provider",
			fmt.Sprintf("--provider must be 'openai' or 'anthropic', got %q", provider),
			"")
	}
	if baseURL == "" {
		return xerrors.User("missing_flag", "--base-url is required", "")
	}
	if model == "" {
		return xerrors.User("missing_flag", "--model is required", "")
	}
	if apiKey == "" && !reuseKey {
		return xerrors.User("missing_flag",
			"--api-key is required (or pass --reuse-api-key to keep the existing one)", "")
	}

	file, _ := config.LoadFile()
	file = config.MigrateLegacy(file)

	if reuseKey {
		existing := lookupExistingKey(file, provider)
		if existing == "" {
			return xerrors.User("no_existing_key",
				fmt.Sprintf("--reuse-api-key set but no existing key for provider %q", provider),
				"run again without --reuse-api-key and pass --api-key")
		}
		apiKey = existing
	}

	entry := &config.ProviderEntry{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
	}
	switch provider {
	case "openai":
		file.OpenAI = entry
	case "anthropic":
		file.Anthropic = entry
	}

	if setDefault || file.DefaultProvider == "" {
		file.DefaultProvider = provider
	}

	if err := config.Save(file); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	path, _ := config.Path()
	fmt.Printf("Saved %s to %s (default=%s)\n", provider, path, file.DefaultProvider)
	return nil
}

// lookupExistingKey returns the on-disk API key for provider, or "" if none.
func lookupExistingKey(file config.File, provider string) string {
	switch provider {
	case "openai":
		if file.OpenAI != nil {
			return file.OpenAI.APIKey
		}
	case "anthropic":
		if file.Anthropic != nil {
			return file.Anthropic.APIKey
		}
	}
	return ""
}

// pendingProvider is one entry in the pending setup file.
type pendingProvider struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// pendingFile is the on-disk shape of ~/.kizunax/.pending-setup.json.
type pendingFile struct {
	DefaultProvider string            `json:"default_provider"`
	Providers       []pendingProvider `json:"providers"`
}

// pendingPath returns the absolute path to the pending setup file.
func pendingPath() (string, error) {
	configPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), ".pending-setup.json"), nil
}

// loadPending reads and parses the pending file. Returns os.ErrNotExist if absent.
func loadPending() (pendingFile, error) {
	var pf pendingFile
	path, err := pendingPath()
	if err != nil {
		return pf, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return pf, err
	}
	if err := json.Unmarshal(data, &pf); err != nil {
		return pf, err
	}
	return pf, nil
}

// deletePending removes the pending file if present. Returns nil if already gone.
func deletePending() error {
	path, err := pendingPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// setupApply reads the pending setup file, resolves a key for each pending
// provider from --key / --<provider>-key / --reuse, writes the final config,
// and deletes the pending file when ALL pending providers were applied.
func setupApply(args []string) error {
	pf, err := loadPending()
	if err != nil {
		if os.IsNotExist(err) {
			return xerrors.User("no_pending",
				"no pending setup found",
				"run /kizunax:setup first")
		}
		return xerrors.User("bad_pending", fmt.Sprintf("cannot read pending file: %v", err), "")
	}
	if len(pf.Providers) == 0 {
		return xerrors.User("empty_pending", "pending file lists no providers", "delete it with --clear-pending and re-run /kizunax:setup")
	}

	sharedKey := flagValue(args, "--key")
	openaiKey := flagValue(args, "--openai-key")
	anthropicKey := flagValue(args, "--anthropic-key")
	reuse := hasFlag(args, "--reuse")

	if sharedKey == "" && openaiKey == "" && anthropicKey == "" && !reuse {
		return xerrors.User("missing_flag",
			"at least one of --key, --openai-key, --anthropic-key, or --reuse is required",
			"")
	}

	file, _ := config.LoadFile()
	file = config.MigrateLegacy(file)

	var applied []string
	var remaining []pendingProvider
	for _, p := range pf.Providers {
		key := ""
		switch p.Name {
		case "openai":
			if openaiKey != "" {
				key = openaiKey
			}
		case "anthropic":
			if anthropicKey != "" {
				key = anthropicKey
			}
		}
		if key == "" && sharedKey != "" {
			key = sharedKey
		}
		if key == "" && reuse {
			key = lookupExistingKey(file, p.Name)
			if key == "" {
				remaining = append(remaining, p)
				continue
			}
		}
		if key == "" {
			remaining = append(remaining, p)
			continue
		}

		entry := &config.ProviderEntry{
			BaseURL: p.BaseURL,
			Model:   p.Model,
			APIKey:  key,
		}
		switch p.Name {
		case "openai":
			file.OpenAI = entry
		case "anthropic":
			file.Anthropic = entry
		default:
			return xerrors.User("bad_provider",
				fmt.Sprintf("pending file lists unknown provider %q", p.Name),
				"")
		}
		applied = append(applied, p.Name)
	}

	if len(applied) == 0 {
		return xerrors.User("no_key_resolved",
			"no provider could be resolved with the given flags",
			"pass --key, a --<provider>-key, or --reuse")
	}

	// Update default to whichever pending provider was first AND got applied.
	for _, p := range pf.Providers {
		if contains(applied, p.Name) {
			if pf.DefaultProvider == p.Name || file.DefaultProvider == "" {
				file.DefaultProvider = p.Name
				break
			}
		}
	}
	if file.DefaultProvider == "" && pf.DefaultProvider != "" {
		file.DefaultProvider = pf.DefaultProvider
	}

	if err := config.Save(file); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	if len(remaining) == 0 {
		_ = deletePending()
		path, _ := config.Path()
		fmt.Printf("Applied %s to %s. Default provider: %s.\n",
			strings.Join(applied, ", "), path, file.DefaultProvider)
		return nil
	}

	// Some providers still need keys — keep pending file with only the unresolved ones.
	pf.Providers = remaining
	if pendingPathStr, err := pendingPath(); err == nil {
		if data, mErr := json.MarshalIndent(pf, "", "  "); mErr == nil {
			tmp := pendingPathStr + ".tmp"
			if wErr := os.WriteFile(tmp, data, 0o600); wErr == nil {
				_ = os.Rename(tmp, pendingPathStr)
			}
		}
	}

	var names []string
	for _, p := range remaining {
		names = append(names, p.Name)
	}
	fmt.Printf("Applied %s. Still pending: %s. Run --apply again with a key for those.\n",
		strings.Join(applied, ", "), strings.Join(names, ", "))
	return nil
}

// setupClearPending removes the pending setup file.
func setupClearPending() error {
	if err := deletePending(); err != nil {
		return xerrors.Internal("clear_pending", "cannot remove pending file", err)
	}
	fmt.Println("Cleared pending setup.")
	return nil
}

// contains is a small string-slice membership helper local to this file.
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
