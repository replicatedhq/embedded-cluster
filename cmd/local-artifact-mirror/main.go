package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/urfave/cli/v2"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	name := path.Base(os.Args[0])
	var app = &cli.App{
		Name:     name,
		Usage:    "Run or pull data for the local artifact mirror",
		Commands: []*cli.Command{serveCommand, pullCommand},
	}
	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
