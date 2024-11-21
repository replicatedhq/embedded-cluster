package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/mattn/go-isatty"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli"
	"github.com/replicatedhq/embedded-cluster/pkg/logging"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

func main() {
	ctx := context.Background()

	logging.SetupLogging()

	prompts.SetTerminal(isatty.IsTerminal(os.Stdout.Fd()))

	name := path.Base(os.Args[0])

	InitAndExecute(ctx, name)
}

func InitAndExecute(ctx context.Context, name string) {
	if err := cli.RootCmd(ctx, name).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
