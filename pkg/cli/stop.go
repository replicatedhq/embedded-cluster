package cli

import (
	"fmt"
	"os"

	"github.com/kardianos/service"
	"github.com/replicatedhq/helmbin/pkg/install"
	"github.com/spf13/cobra"
)

// NewCmdStop returns a cobra command for stopping the systemd service
func NewCmdStop(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the k0s service configured on this host. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("this command must be run as root")
			}
			svc, err := install.InstalledService(cli.Name)
			if err != nil {
				return err
			}
			status, err := svc.Status()
			if err != nil {
				return err
			}
			if status == service.StatusStopped {
				cmd.SilenceUsage = true
				return fmt.Errorf("already stopped")
			}
			return svc.Stop()
		},
	}
	cli.cmdReplaceK0s(cmd)
	return cmd
}
