package cli

import (
	"fmt"
	"os"

	"github.com/kardianos/service"
	"github.com/replicatedhq/helmbin/pkg/install"
	"github.com/spf13/cobra"
)

// NewCmdStart returns a cobra command for starting the systemd service
func NewCmdStart(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the k0s service configured on this host. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("this command must be run as root")
			}
			svc, err := install.InstalledService(cli.Name)
			if err != nil {
				return err
			}
			status, _ := svc.Status()
			if status == service.StatusRunning {
				cmd.SilenceUsage = true
				return fmt.Errorf("already running")
			}
			return svc.Start()
		},
	}
	cli.cmdReplaceK0s(cmd)
	return cmd
}
