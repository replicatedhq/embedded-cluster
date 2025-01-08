package cli

import (
	"fmt"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

// UpgradeJobCmd returns a cobra command for upgrading the embedded cluster operator.
// It is called by KOTS admin console to upgrade the embedded cluster operator and installation.
func UpgradeJobCmd() *cobra.Command {
	var installationFile, previousInstallationVersion string
	var migrateV2 bool

	var installation *ecv1beta1.Installation

	cmd := &cobra.Command{
		Use:          "upgrade-job",
		Short:        "Upgrade k0s and then all addons from within a job that may be restarted",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			installationData, err := readInstallationFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			installation, err = decodeInstallation(installationData)
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Upgrade job version %s started\n", versions.Version)

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			fmt.Printf("Upgrading to installation %s (version %s)\n", installation.Name, installation.Spec.Config.Version)

			if migrateV2 {
				err := runMigrateV2(cmd.Context(), cli, installation)
				if err != nil {
					return fmt.Errorf("failed to migrate v2: %w", err)
				}
				return nil
			}

			i := 0
			sleepDuration := time.Second * 5
			for {
				err = upgrade.Upgrade(cmd.Context(), cli, installation)
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
			return nil
		},
	}

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	cmd.Flags().StringVar(&previousInstallationVersion, "previous-version", "", "the previous installation version")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}
	err = cmd.MarkFlagRequired("previous-version")
	if err != nil {
		panic(err)
	}

	cmd.Flags().BoolVar(&migrateV2, "migrate-v2", false, "Set to true to run the v2 migration")

	return cmd
}
