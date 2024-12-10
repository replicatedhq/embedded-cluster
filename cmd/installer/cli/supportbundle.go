package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	k8syaml "sigs.k8s.io/yaml"
)

func SupportBundleCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "support-bundle",
		Short:         "Generate a support bundle for the embedded-cluster",
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}

			rcutil.InitBestRuntimeConfig(cmd.Context())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
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

			sbPath, err := localLicenseSupportBundleSpec()
			if err != nil {
				return fmt.Errorf("failed to create local license collector: %w", err)
			}

			if sbPath != "" {
				arguments = append(arguments, sbPath)
			}

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

// localLicenseSupportBundleSpec creates a support bundle spec to collect
// the license file used to install embedded-cluster. The file is assumed
// to be in the same directory as the calling binary. If not, we do not
// attempt to collect the license.
func localLicenseSupportBundleSpec() (string, error) {
	// Directory of calling binary
	dir, err := filepath.Abs(path.Dir(path.Clean(os.Args[0])))
	if err != nil {
		return "", err
	}

	// Path to license file
	licensePath := path.Join(dir, "license.yaml")
	_, err = os.Stat(licensePath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("License file not found at %s. Not collecting in support bundle", licensePath)
			return "", nil
		}
		return "", err
	}

	sb := v1beta2.SupportBundle{
		Spec: v1beta2.SupportBundleSpec{
			HostCollectors: []*v1beta2.HostCollect{
				{
					HostCopy: &v1beta2.HostCopy{
						HostCollectorMeta: v1beta2.HostCollectorMeta{
							CollectorName: "embedded-cluster", // Used as directory name in support bundle
						},
						Path: licensePath,
					},
				},
			},
		},
	}

	// return support bundle spec path in temp dir
	sbPath := path.Join(os.TempDir(), "license-support-bundle.yaml")
	d, err := k8syaml.Marshal(sb)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(sbPath, d, 0644)
	if err != nil {
		return "", err
	}

	return sbPath, nil
}
