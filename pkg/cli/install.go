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
	controllerOpts := config.ControllerOptions{}
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs and starts a controller+worker as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), controllerOpts, workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&controllerOpts, &workerOpts, true))

	// TODO
	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	cmd.AddCommand(NewCmdInstallController(cli))

	return cmd
}

// NewCmdInstallController returns a cobra command for installing a controller as a systemd service
func NewCmdInstallController(_ *CLI) *cobra.Command {
	controllerOpts := config.ControllerOptions{}
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Installs and starts a controller as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallController(cmd.Context(), controllerOpts, workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetControllerFlags(&controllerOpts, &workerOpts, false))

	// TODO
	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	return cmd
}

func runInstallController(
	ctx context.Context, controllerOpts config.ControllerOptions, workerOpts config.WorkerOptions,
) error {

	// TODO: options
	config := config.Default()
	// Hack so you can re-run this command
	_ = os.RemoveAll("/etc/systemd/system/k0scontroller.service")

	args := []string{
		"controller",
		fmt.Sprintf("--data-dir=%s", filepath.Join(config.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", config.K0sConfigFile),
	}
	if controllerOpts.EnableWorker {
		args = append(args, "--enable-worker")
	}
	if controllerOpts.NoTaints {
		args = append(args, "--no-taints")
	}
	if workerOpts.TokenFile != "" {
		args = append(args, fmt.Sprintf("--token-file=%s", workerOpts.TokenFile))
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

// NewCmdInstallWorker returns a cobra command for installing a worker as a systemd service
func NewCmdInstallWorker(_ *CLI) *cobra.Command {
	workerOpts := config.WorkerOptions{}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Installs and starts a worker as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallWorker(cmd.Context(), workerOpts)
		},
	}

	cmd.Flags().AddFlagSet(config.GetWorkerFlags(&workerOpts))

	// TODO
	// required for the install command to work
	cmd.Flags().String("config", "", "")
	_ = cmd.Flags().MarkHidden("config")

	return cmd
}

func runInstallWorker(_ context.Context, _ config.WorkerOptions) error {
	// TODO
	return nil
}
