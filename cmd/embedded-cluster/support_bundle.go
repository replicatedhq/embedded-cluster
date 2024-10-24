package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func supportBundleCommand() *cli.Command {
	return &cli.Command{
		Name:  "support-bundle",
		Usage: fmt.Sprintf("Generate a %s support bundle", defaults.BinaryName()),
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
				return fmt.Errorf("unable to find support bundle binary")
			}

			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
			if _, err := os.Stat(hostSupportBundle); err != nil {
				return fmt.Errorf("unable to find host support bundle: %w", err)
			}

			kubeConfig := provider.PathToKubeConfig()
			var env map[string]string
			if _, err := os.Stat(kubeConfig); err == nil {
				// if we have a kubeconfig, use it.
				env = map[string]string{"KUBECONFIG": kubeConfig}
			}

			spin := spinner.Start()
			spin.Infof("Collecting support bundle (this may take a while)")

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{
					Writer:       stdout,
					ErrWriter:    stderr,
					LogOnSuccess: true,
					Env:          env,
				},
				supportBundle,
				"--interactive=false",
				"--load-cluster-specs",
				hostSupportBundle,
			); err != nil {
				spin.Infof("Failed to collect support bundle")
				spin.CloseWithError()
				io.Copy(os.Stdout, stdout)
				io.Copy(os.Stderr, stderr)
				return ErrNothingElseToAdd
			}

			spin.Infof("Support bundle collected!")
			spin.Close()
			return nil
		},
	}
}
