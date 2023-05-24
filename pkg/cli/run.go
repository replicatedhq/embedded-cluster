package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/pkg/controller"
	controllermanager "github.com/emosbaugh/helmbin/pkg/controller/manager"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// CmdRunOptions is a struct to support the run command
type CmdRunOptions struct {
	DataDir string
}

// NewCmdRunOptions returns initialized RunOptions
func NewCmdRunOptions() *CmdRunOptions {
	return &CmdRunOptions{}
}

// NewCmdRun returns a cobra command for running the Kubernetes server
func NewCmdRun(cli *CLI) *cobra.Command {
	o := NewCmdRunOptions()
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"controller"},
		Short:   "Runs the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context(), cli)
		},
	}

	cmd.Flags().StringVar(&o.DataDir, "data-dir", config.DataDirDefault, "Path to the data directory.")
	if err := cmd.MarkFlagDirname("data-dir"); err != nil {
		panic(err)
	}

	// k0s install flags we override temporarily just to get things working
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")
	cmd.Flags().String("enable-worker", "", "")
	_ = cmd.Flags().MarkHidden("enable-worker")
	cmd.Flags().String("no-taints", "", "")
	_ = cmd.Flags().MarkHidden("no-taints")

	return cmd
}

// Complete completes all the required options
func (o *CmdRunOptions) Complete(_ []string) error {
	// nothing to complete
	return nil
}

// Validate validates the provided options
func (o *CmdRunOptions) Validate() error {
	// nothing to validate
	return nil
}

// Run executes the command
func (o *CmdRunOptions) Run(ctx context.Context, _ *CLI) error {
	// TODO: options
	config := config.Default()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	manager := controllermanager.New()
	manager.Add(&controller.Server{
		Config: config,
	})
	manager.Add(&controller.Helm{
		Config: config,
	})
	manager.Add(&controller.K0s{
		Config: config,
	})

	err := manager.Init(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize manager: %w", err)
	}
	logrus.Info("All components initialized")

	err = manager.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}
	logrus.Info("All components started")

	defer func() {
		// Stop Cluster components
		if err := manager.Stop(); err != nil {
			logrus.WithError(err).Error("Failed to stop cluster components")
		} else {
			logrus.Info("All components stopped")
		}
	}()

	<-ctx.Done()
	logrus.Debug("Context done in main")
	logrus.Info("Shutting down controller")

	return nil
}
