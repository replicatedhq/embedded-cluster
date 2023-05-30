package cli

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/pkg/controller"
	"github.com/emosbaugh/helmbin/pkg/controller/manager"
)

// NewCmdRun returns a cobra command for running a combined controller and worker node
func NewCmdRun(cli *CLI) *cobra.Command {
	opts := config.K0sControllerOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs a controller+worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), opts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sControllerFlags(&opts, true))

	cmd.AddCommand(NewCmdRunController(cli))
	cmd.AddCommand(NewCmdRunWorker(cli))

	return cmd
}

// NewCmdRunController returns a cobra command for running a controller node
func NewCmdRunController(_ *CLI) *cobra.Command {
	opts := config.K0sControllerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Runs a controller node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), opts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sControllerFlags(&opts, false))

	return cmd
}

func runController(ctx context.Context, opts config.K0sControllerOptions) error {
	manager := manager.New()
	manager.Add(&controller.K0sController{
		Options: opts,
	})
	manager.Add(&controller.Helm{
		Options: opts.CLIOptions,
	})
	if err := manager.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize manager: %w", err)
	}
	logrus.Info("All components initialized")
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}
	logrus.Info("All components started")
	<-ctx.Done()
	if err := manager.Stop(); err != nil {
		logrus.WithError(err).Error("Failed to stop cluster components")
	}
	logrus.Info("All components stopped")
	logrus.Debug("Context done in main")
	logrus.Info("Shutting down controller")
	return nil
}

// NewCmdRunWorker returns a cobra command for running a worker node
func NewCmdRunWorker(_ *CLI) *cobra.Command {
	opts := config.K0sWorkerOptions{}

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Runs a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorker(cmd.Context(), opts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sWorkerFlags(&opts))

	return cmd
}

func runWorker(_ context.Context, _ config.K0sWorkerOptions) error {
	// TODO
	return nil
}
