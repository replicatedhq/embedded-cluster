package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/cmd"
	"github.com/replicatedhq/embedded-cluster/pkg/logging"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	logging.SetupLogging()

	prompts.SetTerminal(isatty.IsTerminal(os.Stdout.Fd()))

	name := path.Base(os.Args[0])
	app := cmd.NewApp(name)
	if err := app.RunContext(ctx, os.Args); err != nil {
		logrus.Fatal(err)
	}
}
