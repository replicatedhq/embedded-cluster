package cli

import (
	"fmt"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/upgrade"
	"github.com/spf13/cobra"
)

// UpgradeJobCmd returns a cobra command for upgrading the embedded cluster operator.
// It is called by KOTS admin console to upgrade the embedded cluster operator and installation.
func UpgradeJobCmd() *cobra.Command {
	var installationFile, localArtifactMirrorImage string

	cmd := &cobra.Command{
		Use:          "upgrade-job",
		Short:        "Upgrade k0s and then all addons from within a job that may be restarted",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Upgrade command started")

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			installationData, err := readInstallationFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			fmt.Printf("installation data: %s\n", installationData)

			in, err := decodeInstallation(cmd.Context(), []byte(installationData))
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			fmt.Printf("Upgrading to installation %s (k0s version %s)\n", in.Name, in.Spec.Config.Version)

			err = upgrade.Upgrade(cmd.Context(), cli, in, localArtifactMirrorImage)
			if err != nil {
				return fmt.Errorf("failed to upgrade: %w", err)
			}

			fmt.Println("Upgrade completed successfully")
			return nil
		},
	}

	// TODO(upgrade): local-artifact-mirror-image should be included in the installation object
	cmd.Flags().StringVar(&localArtifactMirrorImage, "local-artifact-mirror-image", "", "Local artifact mirror image")

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}

	return cmd
}
