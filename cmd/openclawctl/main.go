package main

import (
	"fmt"
	"os"

	"openclaw-autodeploy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
