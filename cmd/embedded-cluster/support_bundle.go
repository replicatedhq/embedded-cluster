package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
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
			var provider *defaults.Provider
			runtimeConfig, err := configutils.ReadRuntimeConfig()
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("unable to read runtime config: %w", err)
				} else {
					provider = discoverBestProvider(c.Context)
				}
			} else {
				provider = defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			}

			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			supportBundle := provider.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				return fmt.Errorf("unable to find support bundle binary")
			}

			kubeConfig := provider.PathToKubeConfig()
			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")

			spin := spinner.Start()
			spin.Infof("Collecting support bundle (this may take a while)")

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{
					Writer:       stdout,
					ErrWriter:    stderr,
					LogOnSuccess: true,
				},
				supportBundle,
				"--interactive=false",
				fmt.Sprintf("--kubeconfig=%s", kubeConfig),
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
