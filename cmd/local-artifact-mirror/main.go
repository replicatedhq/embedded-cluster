package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	name := path.Base(os.Args[0])

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	// we do this before calling cli.InitAndExecute so that it is set before the process forks
	_ = syscall.Umask(0o022)

	InitAndExecute(ctx, name)
}

func InitAndExecute(ctx context.Context, name string) {
	cli := NewCLI(name)
	err := RootCmd(cli).ExecuteContext(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
