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
	opts := config.K0sControllerOptions{}
	var startService bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs and starts a controller+worker as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), opts, startService)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sControllerFlags(&opts, true))
	cmd.Flags().BoolVar(&startService, "start", true, "Start the service after installation")

	cmd.AddCommand(NewCmdInstallController(cli))
	cmd.AddCommand(NewCmdInstallWorker(cli))

	return cmd
}

// NewCmdInstallController returns a cobra command for installing a controller as a systemd service
func NewCmdInstallController(_ *CLI) *cobra.Command {
	opts := config.K0sControllerOptions{}
	var startService bool

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Installs and starts a controller as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), opts, startService)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sControllerFlags(&opts, false))
	cmd.Flags().BoolVar(&startService, "start", true, "Start the service after installation")

	return cmd
}

func runInstallController(ctx context.Context, opts config.K0sControllerOptions, startService bool) error {

	// Hack so you can re-run this command
	_ = os.RemoveAll("/etc/systemd/system/k0scontroller.service")

	args := []string{
		"controller",
		fmt.Sprintf("--data-dir=%s", filepath.Join(opts.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", opts.CfgFile),
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
	if startService {
		cmd = start.NewStartCmd()
		cmd.SetArgs([]string{})
		if err := cmd.ExecuteContext(ctx); err != nil {
			return fmt.Errorf("failed to start k0s: %w", err)
		}
	}
	return nil
}

// NewCmdInstallWorker returns a cobra command for installing a worker as a systemd service
func NewCmdInstallWorker(_ *CLI) *cobra.Command {
	opts := config.K0sWorkerOptions{}
	var startService bool

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Installs and starts a worker as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallWorker(cmd.Context(), opts, startService)
		},
	}

	cmd.Flags().AddFlagSet(config.GetK0sWorkerFlags(&opts))
	cmd.Flags().BoolVar(&startService, "start", true, "Start the service after installation")

	return cmd
}

func runInstallWorker(_ context.Context, _ config.K0sWorkerOptions, _ bool) error {
	// TODO
	return nil
}
