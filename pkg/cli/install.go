package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/spf13/cobra"
)

// CmdInstallOptions is a struct to support install command
type CmdInstallOptions struct {
	DataDir string
}

// NewCmdInstallOptions returns initialized InstallOptions
func NewCmdInstallOptions() *CmdInstallOptions {
	return &CmdInstallOptions{}
}

// NewCmdInstall returns a cobra command for installing the server as a systemd service
func NewCmdInstall(cli *CLI) *cobra.Command {
	o := NewCmdInstallOptions()
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs the server as a systemd service",
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

	return cmd
}

// Complete completes all the required options
func (o *CmdInstallOptions) Complete(_ []string) error {
	// nothing to complete
	return nil
}

// Validate validates the provided options
func (o *CmdInstallOptions) Validate() error {
	// nothing to validate
	return nil
}

// Run executes the command
func (o *CmdInstallOptions) Run(ctx context.Context, _ *CLI) error {
	// TODO: options
	config := config.Default()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Hack so you can re-run this command
	_ = os.RemoveAll("/etc/systemd/system/k0scontroller.service")

	cmd := install.NewInstallCmd()
	cmd.SetArgs([]string{
		"controller",
		"--enable-worker",
		"--no-taints",
		"--data-dir=" + filepath.Join(config.DataDir, "k0s"),
		"--config=" + config.K0sConfigFile,
	})

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to install k0s: %w", err)
	}

	cmd = start.NewStartCmd()
	cmd.SetArgs([]string{})
	err = cmd.ExecuteContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to start k0s: %w", err)
	}

	return nil
}
