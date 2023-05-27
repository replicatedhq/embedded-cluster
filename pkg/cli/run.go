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
	controllerOpts := config.ControllerOptions{}
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs a controller+worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), controllerOpts, workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&controllerOpts, &workerOpts, true))

	// TODO
	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	cmd.AddCommand(NewCmdRunController(cli))
	cmd.AddCommand(NewCmdRunWorker(cli))

	return cmd
}

// NewCmdRunController returns a cobra command for running a controller node
func NewCmdRunController(_ *CLI) *cobra.Command {
	controllerOpts := config.ControllerOptions{}
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Runs a controller node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), controllerOpts, workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&controllerOpts, &workerOpts, false))

	// TODO
	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	return cmd
}

func runController(
	ctx context.Context, controllerOpts config.ControllerOptions, workerOpts config.WorkerOptions,
) error {

	config := config.Default()
	manager := manager.New()
	manager.Add(&controller.K0sController{
		Config:            config,
		ControllerOptions: controllerOpts,
		WorkerOptions:     workerOpts,
	})
	manager.Add(&controller.Helm{
		Config: config,
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
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Runs a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorker(cmd.Context(), workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetWorkerFlags(&workerOpts))

	return cmd
}

func runWorker(_ context.Context, _ config.WorkerOptions) error {
	// TODO
	return nil
}
