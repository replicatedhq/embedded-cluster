package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
)

var joinCommand = &cli.Command{
	Name:  "join",
	Usage: "Join the current node to an existing cluster",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "role",
			Usage: "The role of the node (can be controller or worker)",
			Value: "worker",
		},
	},
	Action: func(c *cli.Context) error {
		if err := canRunJoin(c); err != nil {
			return err
		}
		logrus.Infof("Materializing binaries")
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize binaries: %w", err)
		}
		logrus.Infof("Saving token to disk")
		if err := saveTokenToDisk(c.Args().First()); err != nil {
			return fmt.Errorf("unable to save token to disk: %w", err)
		}
		logrus.Infof("Installing binary")
		if err := installK0sBinary(); err != nil {
			return fmt.Errorf("unable to install k0s binary: %w", err)
		}
		logrus.Infof("Joining node to cluster")
		if err := runK0sInstallCommand(c.String("role")); err != nil {
			return fmt.Errorf("unable to join node to cluster: %w", err)
		}
		logrus.Infof("Creating systemd unit file")
		if err := createSystemdUnitFile(c.String("role")); err != nil {
			return fmt.Errorf("unable to create systemd unit file: %w", err)
		}
		logrus.Infof("Starting service")
		if err := startK0sService(); err != nil {
			return fmt.Errorf("unable to start service: %w", err)
		}
		return nil
	},
}

// saveTokenToDisk saves the provided token in "/etc/k0s/join-token".
func saveTokenToDisk(token string) error {
	if err := os.MkdirAll("/etc/k0s", 0755); err != nil {
		return err
	}
	data := []byte(token)
	if err := os.WriteFile("/etc/k0s/join-token", data, 0644); err != nil {
		return err
	}
	return nil
}

// installK0sBinary saves the embedded k0s binary to disk under /usr/local/bin.
func installK0sBinary() error {
	in, err := os.Open(defaults.K0sBinaryPath())
	if err != nil {
		return fmt.Errorf("unable to open embedded k0s binary: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile("/usr/local/bin/k0s", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to open k0s binary: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("unable to copy k0s binary: %w", err)
	}
	return nil
}

// startK0sService starts the k0s service.
func startK0sService() error {
	cmd := exec.Command("/usr/local/bin/k0s", "start")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("service start failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// canRunJoin checks if we can run the join command. Checks if we are running on linux,
// if we are root, and if a token has been provided through the command line.
func canRunJoin(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("join command is only supported on linux")
	}
	if os.Getuid() != 0 {
		return fmt.Errorf("join command must be run as root")
	}
	if c.Args().Len() != 1 {
		return fmt.Errorf("usage: %s node join <token>", defaults.BinaryName())
	}
	if role := c.String("role"); role != "controller" && role != "worker" {
		return fmt.Errorf("role must be either controller or worker")
	}
	return nil
}

// createSystemdUnitFile links the k0s systemd unit file.
func createSystemdUnitFile(role string) error {
	dst := fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
	if _, err := os.Stat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	src := "/etc/systemd/system/k0scontroller.service"
	if role == "worker" {
		src = "/etc/systemd/system/k0sworker.service"
	}
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	cmd := exec.Command("systemctl", "daemon-reload")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("systemctl reload failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// runK0sInstallCommand runs the 'k0s install' command using the provided role.
func runK0sInstallCommand(role string) error {
	a := []string{"install", role, "--token-file", "/etc/k0s/join-token", "--force"}
	if role == "controller" {
		a = append(a, "--enable-worker")
	}
	cmd := exec.Command("/usr/local/bin/k0s", a...)
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
