package main

import (
	"context"
	"os"
	"path"

	"github.com/mattn/go-isatty"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

func main() {
	ctx := context.Background()

	cli.SetupLogging()

	prompts.SetTerminal(isatty.IsTerminal(os.Stdout.Fd()))

	name := path.Base(os.Args[0])

	cli.InitAndExecute(ctx, name)
}
