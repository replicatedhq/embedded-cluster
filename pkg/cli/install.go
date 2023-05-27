package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/spf13/cobra"
)

// NewCmdInstall returns a cobra command for installing the server as a systemd service
func NewCmdInstall(_ *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Installs and starts the server as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd.Context(), args)
		},
	}
}

func runInstall(ctx context.Context, args []string) error {
	// TODO: options
	config := config.Default()
	// Hack so you can re-run this command
	_ = os.RemoveAll("/etc/systemd/system/k0scontroller.service")
	cmd := install.NewInstallCmd()
	cmd.SetArgs([]string{
		"controller",
		"--enable-worker",
		"--no-taints",
		fmt.Sprintf("--data-dir=%s", filepath.Join(config.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", config.K0sConfigFile),
	})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("failed to install k0s: %w", err)
	}
	cmd = start.NewStartCmd()
	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("failed to start k0s: %w", err)
	}
	return nil
}
