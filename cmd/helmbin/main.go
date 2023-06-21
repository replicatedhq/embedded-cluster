/*
Package main is the entrypoint for the helmbin binary
*/
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/replicatedhq/helmbin/pkg/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := cli.NewDefaultRootCommand().ExecuteContext(ctx); err != nil {
		log.Fatalf("error executing command: %v", err)
	}
}
