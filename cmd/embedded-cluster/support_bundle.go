package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

type collectOutput struct {
	ArchivePath string `json:"archivePath"`
}

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

			output := bytes.NewBuffer(nil)
			kubeConfig := provider.PathToKubeConfig()
			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")

			spin := spinner.Start()
			spin.Infof("Collecting support bundle (this may take a while)")

			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{Writer: output, ErrWriter: output},
				supportBundle,
				"--interactive=false",
				fmt.Sprintf("--kubeconfig=%s", kubeConfig),
				"--load-cluster-specs",
				hostSupportBundle,
			); err != nil {
				spin.Infof("Failed to collect support bundle")
				spin.CloseWithError()
				io.Copy(os.Stderr, output)
				return ErrNothingElseToAdd
			}

			spin.Infof("Support bundle collected!")
			spin.Close()

			var parsedOutput collectOutput
			if err := json.Unmarshal(output.Bytes(), &parsedOutput); err != nil {
				return fmt.Errorf("unable to parse support bundle output: %w", err)
			}

			logrus.Infof("Support bundle saved to %s", parsedOutput.ArchivePath)
			return nil
		},
	}
}
