package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/index"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

// runIndex routes `kizunax index <subcommand>` invocations.
func runIndex(args []string) error {
	return runIndexCommand(context.Background(), args, os.Stdout)
}

func runIndexCommand(ctx context.Context, args []string, stdout io.Writer) error {
	_ = ctx
	if len(args) == 0 {
		return xerrors.User("index_no_subcmd",
			"missing subcommand: status | sync | purge | info | enable | disable | toggle",
			"e.g. kizunax index status")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Internal("getcwd", "cannot resolve cwd", err)
	}
	ws, err := state.Resolve(cwd)
	if err != nil {
		return xerrors.Internal("resolve_ws", "cannot resolve workspace", err)
	}

	switch args[0] {
	case "status":
		return runIndexStatus(ws, cwd, stdout)
	case "sync":
		return runIndexSync(ws, cwd, stdout)
	case "purge":
		return runIndexPurge(ws, stdout)
	case "info":
		if len(args) < 2 {
			return xerrors.User("index_info_no_symbol",
				"missing symbol name",
				"e.g. kizunax index info Authenticate")
		}
		return runIndexInfo(ws, args[1], stdout)
	case "enable":
		return runIndexEnable(ws, cwd, stdout)
	case "disable":
		return runIndexDisable(ws, cwd, stdout)
	case "toggle":
		return runIndexToggle(ws, cwd, stdout)
	default:
		return xerrors.User("index_unknown",
			fmt.Sprintf("unknown subcommand: %s", args[0]),
			"available: status | sync | purge | info | enable | disable | toggle")
	}
}

func runIndexStatus(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	// Show the current persistent flag state at the top so users can
	// see whether v2 is enabled even when no index has been built yet.
	flagState, _ := state.LoadUseIndex(ws)
	envOverride := os.Getenv("KIZUNAX_USE_INDEX") == "1"
	envKill := os.Getenv("KIZUNAX_DISABLE_INDEX") == "1"
	effective := !envKill && (envOverride || flagState.Enabled)
	fmt.Fprintf(stdout, "Resolver flag:    %s (file=%t, env override=%t, kill switch=%t)\n",
		boolOnOff(effective), flagState.Enabled, envOverride, envKill)
	fmt.Fprintln(stdout, "")

	idxPath := filepath.Join(ws.Root, "index", "index.json")
	info, err := os.Stat(idxPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stdout, "Workspace: %s\nIndex: not built yet — run a review with KIZUNAX_USE_INDEX=1 or `kizunax index sync`.\n", root)
			return nil
		}
		return xerrors.Internal("stat_index", "cannot stat index", err)
	}
	idx, err := index.LoadJSON(idxPath)
	if err != nil {
		fmt.Fprintf(stdout, "Workspace: %s\nIndex: %s (%.1f KB) — load error: %v\n",
			root, idxPath, float64(info.Size())/1024, err)
		return nil
	}

	byLang := map[string]int{}
	totalDefs, totalRefs := 0, 0
	for _, fi := range idx.Files {
		byLang[fi.Lang]++
		totalDefs += len(fi.Defs)
		totalRefs += len(fi.Refs)
	}
	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	sort.Strings(langs)

	fmt.Fprintf(stdout, "Workspace:        %s\n", root)
	fmt.Fprintf(stdout, "Index path:       %s\n", idxPath)
	fmt.Fprintf(stdout, "Index size:       %.1f KB\n", float64(info.Size())/1024)
	fmt.Fprintf(stdout, "Schema version:   %d\n", idx.Version)
	fmt.Fprintf(stdout, "Last full build:  %s\n", time.Unix(0, idx.Built).Format(time.RFC3339))
	fmt.Fprintf(stdout, "Files indexed:    %d\n", len(idx.Files))
	for _, l := range langs {
		fmt.Fprintf(stdout, "  %-12s %d\n", l+":", byLang[l])
	}
	fmt.Fprintf(stdout, "Symbols:          %d defs / %d refs\n", totalDefs, totalRefs)
	return nil
}

func runIndexSync(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Force rebuild of index for %s ...\n", root)
	idxPath := filepath.Join(ws.Root, "index", "index.json")
	_ = os.Remove(idxPath)
	idx, err := index.LoadOrBuild(ws.Root, root)
	if err != nil {
		return xerrors.Internal("index_sync", "rebuild failed", err)
	}
	fmt.Fprintf(stdout, "Done. %d files indexed.\n", len(idx.Files))
	return nil
}

func runIndexPurge(ws state.WorkspaceDir, stdout io.Writer) error {
	dir := filepath.Join(ws.Root, "index")
	if err := os.RemoveAll(dir); err != nil {
		return xerrors.Internal("index_purge", "remove failed", err)
	}
	fmt.Fprintf(stdout, "Purged %s\n", dir)
	return nil
}

func runIndexEnable(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	s, _ := state.LoadUseIndex(ws)
	s.Enabled = true
	if err := state.SaveUseIndex(ws, s); err != nil {
		return xerrors.Internal("save_use_index", "cannot persist flag", err)
	}
	fmt.Fprintf(stdout, "Kizunax index resolver: enabled for workspace %s\n", root)
	fmt.Fprintln(stdout, "Next review will use v2 (index-backed) resolver. First run rebuilds the index (~2 min for a medium repo); subsequent reviews use incremental update.")
	return nil
}

func runIndexDisable(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	s, _ := state.LoadUseIndex(ws)
	s.Enabled = false
	if err := state.SaveUseIndex(ws, s); err != nil {
		return xerrors.Internal("save_use_index", "cannot persist flag", err)
	}
	fmt.Fprintf(stdout, "Kizunax index resolver: disabled for workspace %s\n", root)
	fmt.Fprintln(stdout, "Next review will use v1 (regex) resolver. The on-disk index is kept; run `kizunax index purge` to remove it.")
	return nil
}

func runIndexToggle(ws state.WorkspaceDir, root string, stdout io.Writer) error {
	s, _ := state.LoadUseIndex(ws)
	s.Enabled = !s.Enabled
	if err := state.SaveUseIndex(ws, s); err != nil {
		return xerrors.Internal("save_use_index", "cannot persist flag", err)
	}
	label := "disabled"
	if s.Enabled {
		label = "enabled"
	}
	fmt.Fprintf(stdout, "Kizunax index resolver: %s for workspace %s\n", label, root)
	return nil
}

func runIndexInfo(ws state.WorkspaceDir, symbol string, stdout io.Writer) error {
	idxPath := filepath.Join(ws.Root, "index", "index.json")
	idx, err := index.LoadJSON(idxPath)
	if err != nil {
		return xerrors.User("index_info_no_index",
			fmt.Sprintf("no index found at %s — run `kizunax index sync` first", idxPath),
			"")
	}
	defs := idx.LookupDefs(symbol, "")
	refs := idx.LookupRefs(symbol, "")
	if len(defs) == 0 && len(refs) == 0 {
		fmt.Fprintf(stdout, "Symbol %q not found in index.\n", symbol)
		return nil
	}
	fmt.Fprintf(stdout, "Symbol: %s\n\n", symbol)
	if len(defs) > 0 {
		fmt.Fprintln(stdout, "Definitions:")
		for _, d := range defs {
			out, _ := json.Marshal(d)
			fmt.Fprintf(stdout, "  %s\n", out)
		}
	}
	if len(refs) > 0 {
		fmt.Fprintf(stdout, "\nReferences (%d):\n", len(refs))
		for _, r := range refs {
			out, _ := json.Marshal(r)
			fmt.Fprintf(stdout, "  %s\n", out)
		}
	}
	return nil
}

func boolOnOff(b bool) string {
	if b {
		return "ENABLED (v2)"
	}
	return "disabled (v1)"
}
