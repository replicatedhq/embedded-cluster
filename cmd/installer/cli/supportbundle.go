package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/spf13/cobra"
)

func SupportBundleCmd(ctx context.Context) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "support-bundle",
		Short: "Generate a support bundle",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}

			rc = rcutil.InitBestRuntimeConfig(cmd.Context())
			os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			supportBundle := rc.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				return errors.New("support-bundle command can only be run after an install attempt")
			}

			hostSupportBundle := rc.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
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

			kubeConfig := rc.PathToKubeConfig()
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
					Env:          map[string]string{"TROUBLESHOOT_AUTO_UPDATE": "false"},
					LogOnSuccess: true,
				},
				supportBundle,
				arguments...,
			); err != nil {
				spin.ErrorClosef("Failed to generate support bundle")
				io.Copy(os.Stdout, stdout)
				io.Copy(os.Stderr, stderr)
				return NewErrorNothingElseToAdd(errors.New("failed to generate support bundle"))
			}

			spin.Closef("Support bundle saved at %s", destination)
			return nil
		},
	}

	return cmd
}
