package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
)

const environmentUsageText = `
This script uses the following environment variables:
- REGISTRY_SERVER: the registry server to push the chart/image to (only used for authentication in the case of charts, e.g. index.docker.io)
- REGISTRY_USER: the username to authenticate with.
- REGISTRY_PASS: the password to authenticate with.
- DESTINATION: the destination repository to push the chart to (e.g. ttl.sh/embedded-cluster-charts)
`

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	var app = &cli.App{
		Name:  "buildtools",
		Usage: "Provide a set of tools for building embedded cluster binarires",
		Commands: []*cli.Command{
			updateCommand,
		},
	}
	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
