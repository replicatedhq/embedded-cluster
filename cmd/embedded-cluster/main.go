package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/logging"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	logging.SetupLogging()
	name := path.Base(os.Args[0])
	var app = &cli.App{
		Name:  name,
		Usage: fmt.Sprintf("Install and manage %s", name),
		Commands: []*cli.Command{
			installCommand,
			shellCommand,
			nodeCommands,
			versionCommand,
			joinCommand,
			resetCommand,
			materializeCommand,
		},
	}
	if err := app.RunContext(ctx, os.Args); err != nil {
		logrus.Fatal(err)
	}
}
