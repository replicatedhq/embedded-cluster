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

// NewCmdInstall returns a cobra command for installing a controller+worker as a systemd service
func NewCmdInstall(cli *CLI) *cobra.Command {
	o := config.ControllerOptions{}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs and starts a controller+worker as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), o)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&o, true))

	cmd.AddCommand(NewCmdInstallController(cli))

	return cmd
}

// NewCmdInstallController returns a cobra command for installing a controller as a systemd service
func NewCmdInstallController(_ *CLI) *cobra.Command {
	o := config.ControllerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Installs and starts a controller as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), o)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&o, false))

	return cmd
}

func runInstallController(ctx context.Context, opts config.ControllerOptions) error {
	// TODO: options
	config := config.Default()
	// Hack so you can re-run this command
	_ = os.RemoveAll("/etc/systemd/system/k0scontroller.service")

	args := []string{
		"controller",
		fmt.Sprintf("--data-dir=%s", filepath.Join(config.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", config.K0sConfigFile),
	}
	if opts.EnableWorker {
		args = append(args, "--enable-worker")
	}
	if opts.NoTaints {
		args = append(args, "--no-taints")
	}
	if opts.TokenFile != "" {
		args = append(args, fmt.Sprintf("--token-file=%s", opts.TokenFile))
	}

	cmd := install.NewInstallCmd()
	cmd.SetArgs(args)
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
