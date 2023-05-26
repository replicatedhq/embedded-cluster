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
	o := config.ControllerOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs a controller+worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), o)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&o, true))

	cmd.AddCommand(NewCmdRunController(cli))

	return cmd
}

// NewCmdRunController returns a cobra command for running a controller node
func NewCmdRunController(_ *CLI) *cobra.Command {
	o := config.ControllerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Runs a controller node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(cmd.Context(), o)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&o, false))

	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	return cmd
}

func runController(ctx context.Context, opts config.ControllerOptions) error {
	config := config.Default()
	manager := manager.New()
	manager.Add(&controller.K0sController{
		Config:            config,
		ControllerOptions: opts,
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
