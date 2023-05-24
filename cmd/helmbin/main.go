/*
Package main is the entrypoint for the helmbin binary
*/
package main

import (
	"fmt"
	"os"

	"github.com/emosbaugh/helmbin/pkg/cli"
)

func main() {
	err := cli.NewDefaultRootCommand().Execute() // TODO: ExecuteContext
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
