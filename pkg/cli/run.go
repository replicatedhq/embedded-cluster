package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/pkg/controller"
	"github.com/emosbaugh/helmbin/pkg/controller/manager"
)

// NewCmdRun returns a cobra command for running the Kubernetes server
func NewCmdRun(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Runs the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := config.Default()
			manager := manager.New()
			manager.Add(&controller.K0s{
				Config: config,
			})
			manager.Add(&controller.Helm{
				Config: config,
			})
			if err := manager.Init(cmd.Context()); err != nil {
				return fmt.Errorf("failed to initialize manager: %w", err)
			}
			logrus.Info("All components initialized")
			if err := manager.Start(cmd.Context()); err != nil {
				return fmt.Errorf("failed to start manager: %w", err)
			}
			logrus.Info("All components started")
			<-cmd.Context().Done()
			if err := manager.Stop(); err != nil {
				logrus.WithError(err).Error("Failed to stop cluster components")
			}
			logrus.Info("All components stopped")
			logrus.Debug("Context done in main")
			logrus.Info("Shutting down controller")
			return nil
		},
	}
}
