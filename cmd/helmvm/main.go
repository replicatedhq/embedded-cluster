package main

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {

	logrus.SetLevel(logrus.WarnLevel)

	name := path.Base(os.Args[0])
	var app = &cli.App{
		Name: name,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "verbosity",
				Usage:   "set verbosity: (warn,info,debug,error)",
				Aliases: []string{"v"},
				Value:   "warn",
				Action: func(ctx *cli.Context, v string) error {
					logLevel, err := logrus.ParseLevel(v)
					if err != nil {
						logrus.Fatal(err)
					}
					logrus.SetLevel(logLevel)
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
