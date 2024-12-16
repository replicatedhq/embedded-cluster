package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	name := path.Base(os.Args[0])

	InitAndExecute(ctx, name)
}

func RootCmd(ctx context.Context, v *viper.Viper, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("The %s cluster manager process", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			os.Exit(1)
			return nil
		},
	}

	cmd.AddCommand(StartCmd(ctx, name))
	cmd.AddCommand(StatusCmd(ctx, name))
	cmd.AddCommand(SendCmd(ctx, name))

	return cmd
}

func InitAndExecute(ctx context.Context, name string) {
	v := viper.GetViper()
	if err := RootCmd(ctx, v, name).Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
