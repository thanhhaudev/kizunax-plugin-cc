//go:build !lite

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/grammars"
)

func runGrammars(args []string) error {
	if len(args) == 0 {
		printGrammarsUsage()
		return nil
	}
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "install":
		return runGrammarsInstall(rest)
	case "list":
		return runGrammarsList(rest)
	case "remove":
		return runGrammarsRemove(rest)
	case "-h", "--help", "help":
		printGrammarsUsage()
		return nil
	default:
		printGrammarsUsage()
		return fmt.Errorf("unknown subcommand %q", sub)
	}
}

func printGrammarsUsage() {
	fmt.Fprintf(os.Stderr, `kizunax grammars — manage tree-sitter grammars

Usage:
  kizunax grammars install <lang>             install to ~/.kizunax/grammars/
  kizunax grammars install <lang> --project   install to ./.kizunax/grammars/
  kizunax grammars install --all              install all registered langs (global)
  kizunax grammars list                       show installed grammars
  kizunax grammars remove <lang>              remove from ~/.kizunax/grammars/
  kizunax grammars remove <lang> --project    remove from ./.kizunax/grammars/

Available languages: %s
`, listRegistryLangs())
}

func listRegistryLangs() string {
	keys := []string{}
	for k := range grammars.Registry {
		keys = append(keys, k)
	}
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}

func runGrammarsInstall(args []string) error {
	project := false
	all := false
	var lang string
	for _, a := range args {
		switch a {
		case "--project":
			project = true
		case "--all":
			all = true
		default:
			lang = a
		}
	}
	ctx := context.Background()
	if all {
		for k := range grammars.Registry {
			if err := grammars.Install(ctx, k, project); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", k, err)
			}
		}
		return nil
	}
	if lang == "" {
		return fmt.Errorf("usage: kizunax grammars install <lang> [--project]")
	}
	return grammars.Install(ctx, lang, project)
}

func runGrammarsList(args []string) error {
	items, err := grammars.List()
	if err != nil {
		return err
	}
	fmt.Println("Lookup order (first match wins):")
	fmt.Println("  1. ./.kizunax/grammars/")
	fmt.Println("  2. ~/.kizunax/grammars/")
	fmt.Println()
	if len(items) == 0 {
		fmt.Println("Installed: (none)")
	} else {
		fmt.Println("Installed:")
		for _, it := range items {
			tag := ""
			if e, ok := grammars.Registry[it.Lang]; ok {
				tag = fmt.Sprintf(" (registry v%s)", e.Version)
			} else {
				tag = " (unknown grammar)"
			}
			fmt.Printf("  %-12s %s [%s]%s\n", it.Lang, it.Path, it.Source, tag)
		}
	}
	fmt.Println()
	registered := []string{}
	for k := range grammars.Registry {
		installed := false
		for _, it := range items {
			if it.Lang == k {
				installed = true
				break
			}
		}
		if !installed {
			registered = append(registered, k)
		}
	}
	if len(registered) > 0 {
		fmt.Printf("Available (not installed): %v\n", registered)
	}
	return nil
}

func runGrammarsRemove(args []string) error {
	project := false
	var lang string
	for _, a := range args {
		switch a {
		case "--project":
			project = true
		default:
			lang = a
		}
	}
	if lang == "" {
		return fmt.Errorf("usage: kizunax grammars remove <lang> [--project]")
	}
	return grammars.Remove(lang, project)
}
