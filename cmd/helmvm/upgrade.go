package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/addons"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
)

func stopHelmVM() error {
	cmd := exec.Command("k0s", "stop")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("install failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// canRunUpgrade checks if we can run the upgrade command. Checks if we are running on
// linux and if we are root. This function also ensures that upgrades can't be run on
// a cluster that has been deployed using a centralized configuration.
func canRunUpgrade(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("upgrade command is only supported on linux")
	}
	if os.Getuid() != 0 {
		return fmt.Errorf("upgrade command must be run as root")
	}
	if _, err := os.Stat(defaults.PathToConfig("k0sctl.yaml")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to read configuration: %w", err)
	}
	if defaults.DecentralizedInstall() {
		return nil
	}
	logrus.Errorf("Attempting to upgrade a single node in a cluster with centralized")
	logrus.Errorf("configuration is not supported. Execute the following command for")
	logrus.Errorf("a proper upgrade:")
	logrus.Errorf("\t%s apply", defaults.BinaryName())
	return fmt.Errorf("command not available")
}

var upgradeCommand = &cli.Command{
	Name:  "upgrade",
	Usage: "Upgrade the local node",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
		&cli.StringSliceFlag{
			Name:  "disable-addon",
			Usage: "Disable addon during upgrade",
		},
	},
	Action: func(c *cli.Context) error {
		if err := canRunUpgrade(c); err != nil {
			return err
		}
		logrus.Infof("Materializing binaries")
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize binaries: %w", err)
		}
		logrus.Infof("Stopping %s", defaults.BinaryName())
		if err := stopHelmVM(); err != nil {
			return fmt.Errorf("unable to stop: %w", err)
		}
		logrus.Infof("Installing binary")
		if err := installK0sBinary(); err != nil {
			return fmt.Errorf("unable to install k0s binary: %w", err)
		}
		logrus.Infof("Starting service")
		if err := startK0sService(); err != nil {
			return fmt.Errorf("unable to start service: %w", err)
		}
		kcfg := defaults.PathToConfig("kubeconfig")
		if _, err := os.Stat(kcfg); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("unable to stat kubeconfig: %w", err)
		}
		os.Setenv("KUBECONFIG", kcfg)
		logrus.Infof("Upgrading addons")
		opts := []addons.Option{}
		if c.Bool("no-prompt") {
			opts = append(opts, addons.WithoutPrompt())
		}
		for _, addon := range c.StringSlice("disable-addon") {
			opts = append(opts, addons.WithoutAddon(addon))
		}
		if err := addons.NewApplier(opts...).Apply(c.Context); err != nil {
			return fmt.Errorf("unable to apply addons: %w", err)
		}
		logrus.Infof("Upgrade complete")
		return nil
	},
}
