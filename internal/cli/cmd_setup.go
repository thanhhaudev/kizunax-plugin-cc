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
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func runSetup(args []string) error {
	check := hasFlag(args, "--check")
	rebuild := hasFlag(args, "--rebuild")
	jsonOut := hasFlag(args, "--json")
	save := hasFlag(args, "--save")
	apply := hasFlag(args, "--apply")
	clearPending := hasFlag(args, "--clear-pending")
	status := hasFlag(args, "--status")
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
	if status {
		return setupStatus()
	}
	if web {
		return setupWeb()
	}
	if hasFlag(args, "--enable-stop-gate") {
		return setStopGateFlag(true)
	}
	if hasFlag(args, "--disable-stop-gate") {
		return setStopGateFlag(false)
	}

	return setupWizard()
}

func setStopGateFlag(enable bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getcwd", "cannot get cwd", err)
	}
	ws, err := state.Resolve(cwd)
	if err != nil {
		return err
	}
	current, _ := state.LoadStopGate(ws)
	current.Enabled = enable
	if err := state.SaveStopGate(ws, current); err != nil {
		return err
	}
	status := "disabled"
	if enable {
		status = "enabled"
	}
	fmt.Printf("Kizunax stop-gate: %s for workspace %s\n", status, cwd)
	return nil
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
	file = config.MigrateLegacy(file)

	if len(file.APIKeys) == 0 {
		fmt.Println("Status: ✗ no API keys configured")
		fmt.Println("Run /kizunax:setup to add one.")
		return nil
	}

	fmt.Printf("Keys:        %d configured (%s)\n", len(file.APIKeys), maskKeys(file.APIKeys))
	rotation := file.Rotation
	if rotation == "" {
		rotation = config.RotationRoundRobin
	}
	fmt.Printf("Rotation:    %s\n", rotation)
	fmt.Printf("OpenAI model:    %s\n", firstNonEmptyCheck(file.OpenAIModel, config.DefaultOpenAIModel))
	fmt.Printf("Anthropic model: %s\n", firstNonEmptyCheck(file.AnthropicModel, config.DefaultAnthropicModel))
	fmt.Println()

	// Probe the requested provider (or both if no override).
	providers := []string{providerOverride}
	if providerOverride == "" {
		providers = []string{"openai", "anthropic"}
	}
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

	// Helper (Public v1) block — v0.11+. Defer resolution to config.Load so
	// the printed values reflect KIZUNAX_HELPER_* env-var overrides too.
	fmt.Println("[helper] (Public v1)")
	if cfg, cfgErr := config.Load(providerOverride); cfgErr == nil {
		fmt.Printf("  base_url: %s\n", cfg.Helper.BaseURL)
		fmt.Printf("  model:    %s\n", cfg.Helper.Model)
		fmt.Printf("  timeout:  %ds\n", cfg.Helper.TimeoutSeconds)
		if file.Helper != nil && len(file.Helper.APIKeys) > 0 {
			fmt.Printf("  keys:     %d configured (dedicated helper pool)\n", len(file.Helper.APIKeys))
		} else {
			fmt.Printf("  keys:     reuse provider pool (%d)\n", len(file.APIKeys))
		}
	} else {
		fmt.Printf("  ✗ %v\n", cfgErr)
	}
	fmt.Println()

	return nil
}

// maskKeys renders the first three keys as masked tokens, eliding the rest.
func maskKeys(keys []string) string {
	parts := make([]string, 0, 3)
	for i, k := range keys {
		if i >= 3 {
			parts = append(parts, fmt.Sprintf("…+%d more", len(keys)-3))
			break
		}
		parts = append(parts, maskKey(k))
	}
	return strings.Join(parts, ", ")
}

func firstNonEmptyCheck(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func setupWizard() error {
	fmt.Println("Kizunax interactive setup")
	fmt.Println("(Press Enter to accept defaults in [brackets].)")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	existing, _ := config.LoadFile()
	existing = config.MigrateLegacy(existing)

	hadKeys := len(existing.APIKeys) > 0

	fmt.Println("Paste one or more KizunaX API keys, one per line.")
	if hadKeys {
		fmt.Printf("(Currently saved: %d key(s). Press Enter on a blank line to keep them.)\n", len(existing.APIKeys))
	} else {
		fmt.Println("(At least one is required. End input with an empty line.)")
	}
	var keys []string
	seen := map[string]bool{}
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if !seen[line] {
			seen[line] = true
			keys = append(keys, line)
		}
	}
	if len(keys) == 0 && hadKeys {
		keys = existing.APIKeys
	}
	if len(keys) == 0 {
		return xerrors.User("no_keys", "no API keys entered", "")
	}

	openaiModel := ask(reader, "OpenAI-compat model", firstNonEmptyCheck(existing.OpenAIModel, config.DefaultOpenAIModel))
	anthropicModel := ask(reader, "Anthropic-compat model", firstNonEmptyCheck(existing.AnthropicModel, config.DefaultAnthropicModel))

	out := config.File{
		APIKeys:        keys,
		Rotation:       config.RotationRoundRobin,
		OpenAIModel:    openaiModel,
		AnthropicModel: anthropicModel,
		Temperature:    existing.Temperature,
		MaxTokens:      existing.MaxTokens,
	}
	if err := config.Save(out); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	path, _ := config.Path()
	fmt.Println()
	fmt.Printf("✓ Saved %s (mode 0600)\n", path)
	fmt.Printf("Keys: %d\n", len(keys))
	fmt.Printf("Rotation: %s\n", config.RotationRoundRobin)
	fmt.Printf("Models: openai=%s, anthropic=%s\n", openaiModel, anthropicModel)
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
		"supported: openai, anthropic")
}

// setupJSON prints the current config inventory as JSON. No prompts.
func setupJSON() error {
	path, _ := config.Path()

	out := struct {
		ConfigPath     string `json:"config_path"`
		KeyCount       int    `json:"key_count"`
		Rotation       string `json:"rotation"`
		OpenAIModel    string `json:"openai_model"`
		AnthropicModel string `json:"anthropic_model"`
	}{
		ConfigPath: path,
		Rotation:   config.RotationRoundRobin,
	}

	file, err := config.LoadFile()
	if err == nil {
		file = config.MigrateLegacy(file)
		out.KeyCount = len(file.APIKeys)
		if file.Rotation != "" {
			out.Rotation = file.Rotation
		}
	}
	out.OpenAIModel = firstNonEmptyCheck(file.OpenAIModel, config.DefaultOpenAIModel)
	out.AnthropicModel = firstNonEmptyCheck(file.AnthropicModel, config.DefaultAnthropicModel)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// setupSave writes a config without prompting.
// Required:  one of --api-key "K1,K2,..." OR --reuse-keys
// Optional:  --rotation round-robin | --openai-model M | --anthropic-model M
func setupSave(args []string) error {
	keysArg := flagValue(args, "--api-key")
	reuseKeys := hasFlag(args, "--reuse-keys")
	rotation := flagValue(args, "--rotation")
	openaiModel := flagValue(args, "--openai-model")
	anthropicModel := flagValue(args, "--anthropic-model")

	existing, _ := config.LoadFile()
	existing = config.MigrateLegacy(existing)

	var keys []string
	switch {
	case keysArg != "":
		keys = parseKeyList(keysArg)
		if len(keys) == 0 {
			return xerrors.User("bad_keys", "--api-key list is empty after trimming", "")
		}
	case reuseKeys:
		if len(existing.APIKeys) == 0 {
			return xerrors.User("no_existing_keys",
				"--reuse-keys set but no existing keys on disk",
				"pass --api-key K1,K2,...")
		}
		keys = existing.APIKeys
	default:
		return xerrors.User("missing_flag",
			"--api-key K1,K2,... is required (or pass --reuse-keys to keep existing)",
			"")
	}

	if rotation == "" {
		rotation = existing.Rotation
	}
	if rotation == "" {
		rotation = config.RotationRoundRobin
	}
	if rotation != config.RotationRoundRobin {
		return xerrors.User("bad_rotation",
			fmt.Sprintf("rotation %q not supported in v0.6.6", rotation),
			"valid: round-robin")
	}
	if openaiModel == "" {
		openaiModel = firstNonEmptyCheck(existing.OpenAIModel, config.DefaultOpenAIModel)
	}
	if anthropicModel == "" {
		anthropicModel = firstNonEmptyCheck(existing.AnthropicModel, config.DefaultAnthropicModel)
	}

	out := config.File{
		APIKeys:        keys,
		Rotation:       rotation,
		OpenAIModel:    openaiModel,
		AnthropicModel: anthropicModel,
		Temperature:    existing.Temperature,
		MaxTokens:      existing.MaxTokens,
	}
	if err := config.Save(out); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}

	path, _ := config.Path()
	fmt.Printf("Saved %d key(s) to %s. Rotation: %s. Models: openai=%s, anthropic=%s.\n",
		len(keys), path, rotation, openaiModel, anthropicModel)
	return nil
}

// parseKeyList splits a comma-separated key string, trims each entry,
// drops blanks, and dedupes while preserving input order.
func parseKeyList(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range strings.Split(s, ",") {
		k := strings.TrimSpace(raw)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// pendingFile is the v0.6.6 pending setup record. Keys are intentionally NOT
// stored here — the slash command writes provider/rotation/models, and the
// user supplies keys at --apply time.
type pendingFile struct {
	Rotation       string `json:"rotation"`
	OpenAIModel    string `json:"openai_model"`
	AnthropicModel string `json:"anthropic_model"`
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

// setupApply reads the pending setup file, applies keys (from --api-key or
// --reuse), writes final config, and removes the pending file.
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

	keysArg := flagValue(args, "--api-key")
	reuse := hasFlag(args, "--reuse")

	existing, _ := config.LoadFile()
	existing = config.MigrateLegacy(existing)

	var keys []string
	switch {
	case keysArg != "":
		keys = parseKeyList(keysArg)
		if len(keys) == 0 {
			return xerrors.User("bad_keys", "--api-key list is empty after trimming", "")
		}
	case reuse:
		if len(existing.APIKeys) == 0 {
			return xerrors.User("no_existing_keys",
				"--reuse set but no existing keys on disk",
				"pass --api-key K1,K2,...")
		}
		keys = existing.APIKeys
	default:
		return xerrors.User("missing_flag",
			"--api-key K1,K2,... is required (or --reuse to keep existing)", "")
	}

	rotation := pf.Rotation
	if rotation == "" {
		rotation = config.RotationRoundRobin
	}
	openaiModel := firstNonEmptyCheck(pf.OpenAIModel, existing.OpenAIModel, config.DefaultOpenAIModel)
	anthropicModel := firstNonEmptyCheck(pf.AnthropicModel, existing.AnthropicModel, config.DefaultAnthropicModel)

	out := config.File{
		APIKeys:        keys,
		Rotation:       rotation,
		OpenAIModel:    openaiModel,
		AnthropicModel: anthropicModel,
		Temperature:    existing.Temperature,
		MaxTokens:      existing.MaxTokens,
	}
	if err := config.Save(out); err != nil {
		return xerrors.Internal("save_config", "cannot save config", err)
	}
	_ = deletePending()

	path, _ := config.Path()
	fmt.Printf("Applied %d key(s) to %s. Rotation: %s.\n", len(keys), path, rotation)
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
