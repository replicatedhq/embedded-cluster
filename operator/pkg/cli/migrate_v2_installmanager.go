package cli

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/manager/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// UpgradeInstallV2ManagerCmd returns a cobra command intended to be run in a pod on all nodes in
// the cluster. It will download the manager binary and install it as a systemd service on the
// host.
func UpgradeInstallV2ManagerCmd() *cobra.Command {
	var installationFile string
	var licenseID string
	var licenseEndpoint string
	var versionLabel string

	cmd := &cobra.Command{
		Use:   "install-v2-manager",
		Short: "Downloads the v2 manager binary and installs it as a systemd service.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetLevel(logrus.DebugLevel)

			installationData, err := readInstallationFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			installation, err := decodeInstallation(installationData)
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := migrate.InstallAndStartManager(cmd.Context(), licenseID, licenseEndpoint, versionLabel); err != nil {
				return fmt.Errorf("failed to run manager migration: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&licenseID, "license-id", "", "The license ID")
	err = cmd.MarkFlagRequired("license-id")
	if err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&licenseEndpoint, "license-endpoint", "", "The license endpoint")
	err = cmd.MarkFlagRequired("license-endpoint")
	if err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&versionLabel, "version-label", "", "The application version label")
	err = cmd.MarkFlagRequired("version-label")
	if err != nil {
		panic(err)
	}

	return cmd
}
