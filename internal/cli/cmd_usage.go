package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/pkg/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/pkg/render"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

// runUsage is `kizunax usage [--provider X] [--verbose]`.
func runUsage(args []string) error {
	verbose := hasFlag(args, "--verbose")
	providerOverride := flagValue(args, "--provider")

	cfg, err := config.Load(providerOverride)
	if err != nil {
		return err
	}

	base, err := usage.DeriveBase(cfg.BaseURL)
	if err != nil {
		return xerrors.User("unknown_endpoint",
			fmt.Sprintf("cannot derive usage endpoint host from base_url %q: %v", cfg.BaseURL, err),
			"check ~/.kizunax/config.json — base_url must be a full URL like https://kizunax.io/api/coding/v1")
	}

	file, err := config.LoadFile()
	if err != nil {
		return err
	}
	file = config.MigrateLegacy(file)
	providerKeys := file.APIKeys
	var helperKeys []string
	if file.Helper != nil {
		helperKeys = file.Helper.APIKeys
	}
	keys := dedupeUsageKeys(providerKeys, helperKeys)
	if len(keys) == 0 {
		return xerrors.User("missing_api_key",
			"no API keys configured",
			"run /kizunax:setup to add keys")
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] provider=%s base=%s keys=%d\n", cfg.Provider, base, len(keys))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	f := usage.NewFetcher(base)
	usages := f.FetchAll(ctx, keys)

	cwd, err := os.Getwd()
	if err == nil {
		if ws, wsErr := state.Resolve(cwd); wsErr == nil {
			_ = usage.SaveCache(ws, usage.Snapshot{Provider: cfg.Provider, Usages: usages})
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] cache skipped: %v\n", wsErr)
		}
	}

	writeUsageOutput(os.Stdout, cfg.Provider, file.Rotation, usages, time.Now())
	return nil
}

// dedupeUsageKeys returns the union of provider + helper keys, preserving
// order (provider keys first, helper keys after). Empty strings and
// duplicates are skipped.
func dedupeUsageKeys(providerKeys, helperKeys []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(providerKeys)+len(helperKeys))
	for _, k := range providerKeys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	for _, k := range helperKeys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func writeUsageOutput(w io.Writer, provider, rotation string, usages []usage.KeyUsage, now time.Time) {
	fmt.Fprintf(w, "Provider: %s\n\n", provider)
	fmt.Fprint(w, render.RenderUsageAt(usage.Snapshot{Provider: provider, Usages: usages}, rotation, now))
}
