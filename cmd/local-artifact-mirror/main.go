package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/replicatedhq/embedded-cluster/cmd/local-artifact-mirror/cli"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	logrus.SetOutput(os.Stdout)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	name := path.Base(os.Args[0])

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	// we do this before calling cli.InitAndExecute so that it is set before the process forks
	_ = syscall.Umask(0o022)

	c := cli.NewCLI(name)
	err := cli.RootCmd(c).ExecuteContext(ctx)
	cobra.CheckErr(err)
}
