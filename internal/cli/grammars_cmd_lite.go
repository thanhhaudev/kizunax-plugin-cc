//go:build lite

package cli

import (
	"fmt"
	"os"
)

func runGrammars(args []string) error {
	fmt.Fprintln(os.Stderr, "kizunax grammars: requires non-lite build (build without -tags lite)")
	return nil
}
