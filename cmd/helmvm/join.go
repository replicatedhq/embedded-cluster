package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
	"github.com/replicatedhq/helmvm/pkg/metrics"
)

var joinCommand = &cli.Command{
	Name:  "join",
	Usage: "Join the current node to an existing cluster",
	Action: func(c *cli.Context) error {
		rawtoken := c.Args().First()
		decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(rawtoken))
		var hvmtoken JoinToken
		if err := json.NewDecoder(decoder).Decode(&hvmtoken); err != nil {
			return fmt.Errorf("unable to decode join token: %w", err)
		}
		metrics.ReportJoinStarted(c.Context, hvmtoken.ClusterID)
		if err := canRunJoin(c); err != nil {
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Materializing binaries")
		if err := goods.Materialize(); err != nil {
			err := fmt.Errorf("unable to materialize binaries: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		if err := runHostPreflightsLocally(c); err != nil {
			err := fmt.Errorf("unable to run host preflights locally: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Saving token to disk")
		if err := saveTokenToDisk(hvmtoken.Token); err != nil {
			err := fmt.Errorf("unable to save token to disk: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Installing binary")
		if err := installK0sBinary(); err != nil {
			err := fmt.Errorf("unable to install k0s binary: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Joining node to cluster")
		if err := runK0sInstallCommand(hvmtoken.Role); err != nil {
			err := fmt.Errorf("unable to join node to cluster: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Creating systemd unit file")
		if err := createSystemdUnitFile(hvmtoken.Role); err != nil {
			err := fmt.Errorf("unable to create systemd unit file: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		logrus.Infof("Starting service")
		if err := startK0sService(); err != nil {
			err := fmt.Errorf("unable to start service: %w", err)
			metrics.ReportJoinFailed(c.Context, hvmtoken.ClusterID, err)
			return err
		}
		metrics.ReportJoinSucceeded(c.Context, hvmtoken.ClusterID)
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
