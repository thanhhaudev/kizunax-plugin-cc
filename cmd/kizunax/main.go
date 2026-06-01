package main

import (
	"fmt"
	"os"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/cli"
)

func main() {
	if err := cli.Dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
