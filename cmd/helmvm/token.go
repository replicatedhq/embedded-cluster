package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

var tokenCommands = &cli.Command{
	Name:        "token",
	Usage:       "Manage node join tokens",
	Subcommands: []*cli.Command{tokenCreateCommand},
}

var tokenCreateCommand = &cli.Command{
	Name:  "create",
	Usage: "Creates a new node join token",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "role",
			Usage: "The role of the token (can be controller or worker)",
			Value: "worker",
		},
		&cli.DurationFlag{
			Name:  "expiry",
			Usage: "For how long the token should be valid",
			Value: 24 * time.Hour,
		},
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
	},
	Action: func(c *cli.Context) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("token create is only supported on linux")
		}
		if os.Getuid() != 0 {
			return fmt.Errorf("token create must be run as root")
		}
		role := c.String("role")
		if role != "worker" && role != "controller" {
			return fmt.Errorf("invalid role %q", role)
		}
		useprompt := !c.Bool("no-prompt")
		cfgpath := defaults.PathToConfig("k0sctl.yaml")
		if _, err := os.Stat(cfgpath); err != nil {
			if os.IsNotExist(err) {
				logrus.Errorf("Unable to find the cluster configuration.")
				logrus.Errorf("Consider running the command using 'sudo -E'")
				return fmt.Errorf("configuration not found")
			}
			return fmt.Errorf("unable to stat k0sctl.yaml: %w", err)
		}
		logrus.Infof("Creating node join token for role %s", role)
		if !defaults.DecentralizedInstall() {
			logrus.Warn("You are opting out of the centralized cluster management.")
			logrus.Warn("Through the centralized management you can manage all your")
			logrus.Warn("cluster nodes from a single location. If you decide to move")
			logrus.Warn("on the centralized management won't be available anymore")
			if useprompt && !prompts.New().Confirm("Do you want to continue ?", true) {
				return nil
			}
		}
		dur := c.Duration("expiry").String()
		buf := bytes.NewBuffer(nil)
		cmd := exec.Command("k0s", "token", "create", "--expiry", dur, "--role", role)
		cmd.Stdout = buf
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}
		if !defaults.DecentralizedInstall() {
			if err := defaults.SetInstallAsDecentralized(); err != nil {
				return fmt.Errorf("failed to set decentralized install: %w", err)
			}
		}
		fmt.Println("Token created successfully.")
		fmt.Printf("This token is valid for %s hours.\n", dur)
		fmt.Println("You can now run the following command in a remote node to add it")
		fmt.Printf("to the cluster as a %q node:\n", role)
		fmt.Printf("%s node join --role %s %s", defaults.BinaryName(), role, buf.String())
		return nil
	},
}
