package main

import (
	"context"
	"os"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func main() {
	ctx := context.Background()

	cli.SetupLogging()

	prompts.SetTerminal(isatty.IsTerminal(os.Stdout.Fd()))

	appSlug := runtimeconfig.BinaryName()

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	// we do this before calling cli.InitAndExecute so that it is set before the process forks
	_ = syscall.Umask(0o022)

	cli.InitAndExecute(ctx, appSlug)
}
