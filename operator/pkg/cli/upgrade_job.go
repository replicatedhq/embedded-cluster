package cli

import (
	"fmt"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

// UpgradeJobCmd returns a cobra command for upgrading the embedded cluster operator.
// It is called by KOTS admin console to upgrade the embedded cluster operator and installation.
func UpgradeJobCmd() *cobra.Command {
	var installationFile, previousInstallationVersion string
	var installation *ecv1beta1.Installation

	var migrateV2 bool
	var migrateV2Secret, appSlug, appVersionLabel string

	cmd := &cobra.Command{
		Use:          "upgrade-job",
		Short:        "Upgrade k0s and then all addons from within a job that may be restarted",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			installation, err = getInstallationFromFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to get installation from file: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(installation.Spec.RuntimeConfig)

			if migrateV2 {
				if migrateV2Secret == "" {
					return fmt.Errorf("--migrate-v2 is set to true but --migrate-v2-secret is not set")
				}
				if appSlug == "" {
					return fmt.Errorf("--migrate-v2 is set to true but --app-slug is not set")
				}
				if appVersionLabel == "" {
					return fmt.Errorf("--migrate-v2 is set to true but --app-version-label is not set")
				}

				manager.SetServiceName(appSlug)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Upgrade job version %s started\n", versions.Version)

			ctx := cmd.Context()

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			fmt.Printf("Upgrading to installation %s (version %s)\n", installation.Name, installation.Spec.Config.Version)

			i := 0
			sleepDuration := time.Second * 5
			for {
				err = upgrade.Upgrade(ctx, cli, installation)
				if err != nil {
					fmt.Printf("Upgrade failed, retrying: %s\n", err.Error())
					if i >= 10 {
						return fmt.Errorf("failed to upgrade after %s", (sleepDuration * time.Duration(i)).String())
					}

					time.Sleep(sleepDuration)
					i++
					continue
				}
				break
			}

			fmt.Println("Upgrade completed successfully")

			if migrateV2 {
				logf := func(format string, args ...any) {
					fmt.Println(fmt.Sprintf(format, args...))
				}

				err = runMigrateV2PodAndWait(ctx, logf, cli, installation, migrateV2Secret, appSlug, appVersionLabel)
				if err != nil {
					return fmt.Errorf("failed to run v2 migration: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&previousInstallationVersion, "previous-version", "", "the previous installation version")
	err = cmd.MarkFlagRequired("previous-version")
	if err != nil {
		panic(err)
	}

	cmd.Flags().BoolVar(&migrateV2, "migrate-v2", false, "Set to true to run the v2 migration")
	cmd.Flags().StringVar(&migrateV2Secret, "migrate-v2-secret", "", "The secret name from which to read the license (required if --migrate-v2 is set to true)")
	cmd.Flags().StringVar(&appSlug, "app-slug", "", "The application slug (required if --migrate-v2 is set to true)")
	cmd.Flags().StringVar(&appVersionLabel, "app-version-label", "", "The application version label (required if --migrate-v2 is set to true)")

	return cmd
}
