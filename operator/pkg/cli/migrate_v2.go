package cli

import (
	"fmt"
	"log"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/cli/migratev2"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

type migrateV2Flags struct {
	installationFile string
	licenseID        string
	licenseEndpoint  string
	versionLabel     string

	installation *ecv1beta1.Installation
}

// MigrateV2Cmd returns a cobra command for migrating the installation from v1 to v2.
func MigrateV2Cmd() *cobra.Command {
	var flags migrateV2Flags

	cmd := &cobra.Command{
		Use:          "migrate-v2",
		Short:        "Migrates the Embedded Cluster installation from v1 to v2",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := flags.Bind()
			if err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(flags.installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			err = migratev2.RunManagerInstallJobsAndWait(ctx, log.Printf, cli, flags.installation, flags.licenseID, flags.licenseEndpoint, flags.versionLabel)
			if err != nil {
				return fmt.Errorf("failed to run manager install jobs: %w", err)
			}

			err = migratev2.CleanupV1(ctx, log.Printf, cli)
			if err != nil {
				return fmt.Errorf("failed to cleanup operator: %w", err)
			}

			return nil
		},
	}

	err := flags.Configure(cmd)
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(
		MigrateV2InstallManagerCmd(),
	)

	return cmd
}

func (m *migrateV2Flags) Configure(cmd *cobra.Command) error {
	cmd.Flags().StringVar(&m.installationFile, "installation", "", "Path to the installation file")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		return err
	}
	cmd.Flags().StringVar(&m.licenseEndpoint, "license-endpoint", "", "The license endpoint (required for v2)")
	err = cmd.MarkFlagRequired("license-endpoint")
	if err != nil {
		return err
	}
	cmd.Flags().StringVar(&m.versionLabel, "version-label", "", "The application version label")
	err = cmd.MarkFlagRequired("version-label")
	if err != nil {
		return err
	}
	return nil
}

func (f *migrateV2Flags) Bind() error {
	f.licenseID = os.Getenv("LICENSE_ID")
	if f.licenseID == "" {
		return fmt.Errorf("LICENSE_ID is not set")
	}

	installation, err := getInstallationFromFile(f.installationFile)
	if err != nil {
		return fmt.Errorf("get installation from file: %w", err)
	}
	f.installation = installation

	return nil
}
