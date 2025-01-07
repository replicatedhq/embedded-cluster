package cli

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/manager/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func MigrateManagerCmd() *cobra.Command {
	var installationFile string
	var licenseID string
	var licenseEndpoint string
	var versionLabel string

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Migrate to the manager service",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetLevel(logrus.DebugLevel)

			if installationFile == "" {
				return fmt.Errorf("installation file is required")
			}

			data, err := os.ReadFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			installation, err := decodeInstallation(data)
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := migrate.Migrate(cmd.Context(), licenseID, licenseEndpoint, versionLabel); err != nil {
				return fmt.Errorf("failed to run manager migration: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&installationFile, "installation-file", "", "The path to the installation file")
	err := cmd.MarkFlagRequired("installation-file")
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
