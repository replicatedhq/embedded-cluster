package main

import (
	"fmt"
	"os"
	"path"

	"github.com/replicatedhq/helmvm/pkg/logging"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {

	logging.SetupLogging()

	name := path.Base(os.Args[0])
	var app = &cli.App{
		Name: name,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "output all setup messages to stdout",
				Aliases: []string{"d"},
				Value:   false,
				Action: func(ctx *cli.Context, v bool) error {
					logging.Debug = v
					return nil
				},
			},
		},
		Usage: fmt.Sprintf("Installs or updates %s.", name),
		Commands: []*cli.Command{
			installCommand,
			bundleCommand,
			embedCommand,
			shellCommand,
			nodeCommands,
			versionCommand,
		},
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}

}
