package cli

import (
	"context"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/spf13/cobra"
)

var (
	runtimeConfig *ecv1beta1.RuntimeConfigSpec
	provider      *defaults.Provider
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

			if os.Getuid() == 0 {
				provider = discoverBestProvider(cmd.Context(), runtimeConfig)

				if runtimeConfig != nil {
					// if there is a data-dir, local-artifact-mirror-port, or admin-console-port flag, we need to set the runtime config
					if cmd.Flags().Lookup("data-dir") != nil ||
						cmd.Flags().Lookup("local-artifact-mirror-port") != nil ||
						cmd.Flags().Lookup("admin-console-port") != nil {
						runtimeConfig = ecv1beta1.GetDefaultRuntimeConfig()
					}

					// apply data-dir, if it's a valid flag
					if cmd.Flags().Lookup("data-dir") != nil {
						v, err := cmd.Flags().GetString("data-dir")
						if err != nil {
							return fmt.Errorf("unable to get data-dir flag: %w", err)
						}
						fmt.Printf("data dir: %s\n", v)

						runtimeConfig.DataDir = v
					}

					// apply local artifact mirror port, if it's a valid flag
					if cmd.Flags().Lookup("local-artifact-mirror-port") != nil {
						v, err := cmd.Flags().GetInt("local-artifact-mirror-port")
						if err != nil {
							return fmt.Errorf("unable to get local-artifact-mirror-port flag: %w", err)
						}
						runtimeConfig.LocalArtifactMirror.Port = v
					}

					// apply admin console port, if it's a valid flag
					if cmd.Flags().Lookup("admin-console-port") != nil {
						v, err := cmd.Flags().GetInt("admin-console-port")
						if err != nil {
							return fmt.Errorf("unable to get admin-console-port flag: %w", err)
						}
						runtimeConfig.AdminConsole.Port = v
					}
				}

				os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())
				os.Setenv("KUBECONFIG", provider.PathToKubeConfig())

				cobra.OnFinalize(func() {
					tryRemoveTmpDirContents(provider)
				})
			}

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

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
