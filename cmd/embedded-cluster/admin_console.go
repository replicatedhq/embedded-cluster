package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
)

func adminConsoleCommand() *cli.Command {
	return &cli.Command{
		Name:  "admin-console",
		Usage: fmt.Sprintf("Manage the %s Admin Console", defaults.BinaryName()),
		Subcommands: []*cli.Command{
			adminConsoleResetPassswordCommand(),
		},
	}
}

func adminConsoleResetPassswordCommand() *cli.Command {
	return &cli.Command{
		Name:  "reset-password",
		Usage: "Reset the Admin Console password",
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("reset-password command must be run as root")
			}
			if len(c.Args().Slice()) != 1 {
				return fmt.Errorf("expected admin console password as argument")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider, err := getProviderFromCluster(c.Context)
			if err != nil {
				return err
			}

			password := c.Args().Get(0)
			if !validateAdminConsolePassword(password, password) {
				return ErrNothingElseToAdd
			}

			if err := kotscli.ResetPassword(provider, password); err != nil {
				return err
			}

			logrus.Info("Admin Console password reset successfully")
			return nil
		},
	}
}
