package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/cmd"
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
	app := cmd.NewApp(name)
	if err := app.RunContext(ctx, os.Args); err != nil {
		logrus.Fatal(err)
	}
}
