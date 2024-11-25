package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	cmdutil "github.com/replicatedhq/embedded-cluster/pkg/cmd/util"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func SupportBundleCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "support-bundle",
		Short: "Generate a support bundle for the embedded-cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}

			cmdutil.InitBestRuntimeConfig(cmd.Context())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			supportBundle := runtimeconfig.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				logrus.Errorf("support-bundle command can only be run after an install attempt")
				return ErrNothingElseToAdd
			}

			hostSupportBundle := runtimeconfig.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
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

			kubeConfig := runtimeconfig.PathToKubeConfig()
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

	return cmd
}
