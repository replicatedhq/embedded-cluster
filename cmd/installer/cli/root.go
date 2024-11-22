package cli

import (
	"context"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	provider *defaults.Provider
)

func RootCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          name,
		Short:        name,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dryrun.Enabled() {
				dryrun.RecordFlags(cmd.Flags())
			}

			// for any command that has an "airgap-bundle" flag, disable metrics
			if cmd.Flags().Lookup("airgap-bundle") != nil {
				v, err := cmd.Flags().GetString("airgap-bundle")
				if err != nil {
					return fmt.Errorf("unable to get airgap-bundle flag: %w", err)
				}

				if v != "" {
					metrics.DisableMetrics()
				}
			}

			if os.Getuid() != 0 {
				return nil
			}

			hasDataDirFlag := cmd.Flags().Lookup("data-dir") != nil
			hasLocalArtifactMirrorPortFlag := cmd.Flags().Lookup("local-artifact-mirror-port") != nil
			hasAdminConsolePortFlag := cmd.Flags().Lookup("admin-console-port") != nil

			if hasDataDirFlag || hasLocalArtifactMirrorPortFlag || hasAdminConsolePortFlag {
				runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()
				provider = defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			} else {
				provider = discoverBestProvider(cmd.Context())
			}

			// apply data-dir, if it's a valid flag
			if hasDataDirFlag {
				v, err := cmd.Flags().GetString("data-dir")
				if err != nil {
					return fmt.Errorf("unable to get data-dir flag: %w", err)
				}
				provider.SetDataDir(v)
			}

			// apply local artifact mirror port, if it's a valid flag
			if hasLocalArtifactMirrorPortFlag {
				v, err := cmd.Flags().GetInt("local-artifact-mirror-port")
				if err != nil {
					return fmt.Errorf("unable to get local-artifact-mirror-port flag: %w", err)
				}
				provider.SetLocalArtifactMirrorPort(v)
			}

			// apply admin console port, if it's a valid flag
			if hasAdminConsolePortFlag {
				v, err := cmd.Flags().GetInt("admin-console-port")
				if err != nil {
					return fmt.Errorf("unable to get admin-console-port flag: %w", err)
				}
				provider.SetAdminConsolePort(v)
			}

			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())
			os.Setenv("KUBECONFIG", provider.PathToKubeConfig())

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if dryrun.Enabled() {
				if err := dryrun.Dump(); err != nil {
					return fmt.Errorf("unable to dump dry run info: %w", err)
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			os.Exit(1)
			return nil
		},
	}

	cmd.AddCommand(InstallCmd(ctx, name))
	cmd.AddCommand(JoinCmd(ctx, name))
	cmd.AddCommand(ShellCmd(ctx, name))
	cmd.AddCommand(NodeCmd(ctx, name))
	cmd.AddCommand(VersionCmd(ctx, name))
	cmd.AddCommand(ResetCmd(ctx, name))
	cmd.AddCommand(MaterializeCmd(ctx, name))
	cmd.AddCommand(UpdateCmd(ctx, name))
	cmd.AddCommand(RestoreCmd(ctx, name))
	cmd.AddCommand(AdminConsoleCmd(ctx, name))
	cmd.AddCommand(SupportBundleCmd(ctx, name))

	return cmd
}

func TryRemoveTmpDirContents() {
	if provider == nil {
		return
	}
	if err := helpers.RemoveAll(provider.EmbeddedClusterTmpSubDir()); err != nil {
		logrus.Debugf("failed to remove tmp dir contents: %v", err)
	}
}
