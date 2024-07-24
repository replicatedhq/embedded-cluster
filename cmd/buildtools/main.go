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
- CHARTS_REGISTRY_SERVER: the registry server to push the chart to (e.g. index.docker.io)
- CHARTS_REGISTRY_USER: the username to authenticate with.
- CHARTS_REGISTRY_PASS: the password to authenticate with.
- IMAGES_REGISTRY_SERVER: the registry server to push the images to (e.g. index.docker.io)
- IMAGES_REGISTRY_USER: the username to authenticate with.
- IMAGES_REGISTRY_PASS: the password to authenticate with.
- CHARTS_DESTINATION: the destination repository to push the chart to (e.g. ttl.sh/embedded-cluster-charts)
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
