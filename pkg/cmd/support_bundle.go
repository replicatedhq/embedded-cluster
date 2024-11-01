package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func supportBundleCommand() *cli.Command {
	return &cli.Command{
		Name:  "support-bundle",
		Usage: fmt.Sprintf("Generate a support bundle for %s", defaults.BinaryName()),
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider := discoverBestProvider(c.Context)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			supportBundle := provider.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				logrus.Errorf("support-bundle command can only be run after an install attempt")
				return ErrNothingElseToAdd
			}

			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
			if _, err := os.Stat(hostSupportBundle); err != nil {
				return fmt.Errorf("unable to find host support bundle: %w", err)
			}

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get current working directory: %w", err)
			}
			now := time.Now().Format("2006-01-02T15_04_05")
			fname := fmt.Sprintf("support-bundle-%s.tar.gz", now)
			destination := filepath.Join(pwd, fname)

			kubeConfig := provider.PathToKubeConfig()
			arguments := []string{}
			if _, err := os.Stat(kubeConfig); err == nil {
				arguments = append(arguments, fmt.Sprintf("--kubeconfig=%s", kubeConfig))
			}

			arguments = append(
				arguments,
				"--interactive=false",
				"--load-cluster-specs",
				fmt.Sprintf("--output=%s", destination),
				hostSupportBundle,
			)

			spin := spinner.Start()
			spin.Infof("Generating support bundle (this can take a while)")

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{
					Stdout:       stdout,
					Stderr:       stderr,
					LogOnSuccess: true,
				},
				supportBundle,
				arguments...,
			); err != nil {
				spin.Infof("Failed to generate support bundle")
				spin.CloseWithError()
				io.Copy(os.Stdout, stdout)
				io.Copy(os.Stderr, stderr)
				return ErrNothingElseToAdd
			}

			spin.Infof("Support bundle saved at %s", destination)
			spin.Close()
			return nil
		},
	}
}