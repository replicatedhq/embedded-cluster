package main

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	name := path.Base(os.Args[0])
	var app = &cli.App{
		Name:  name,
		Usage: fmt.Sprintf("Installs or updates %s.", name),
		Commands: []*cli.Command{
			installCommand,
			bundleCommand,
			embedCommand,
			shellCommand,
			nodeCommands,
		},
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
