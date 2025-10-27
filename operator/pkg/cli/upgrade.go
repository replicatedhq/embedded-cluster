package cli

import (
	"fmt"
	"io"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// UpgradeCmd returns a cobra command for creating a job to upgrade the embedded cluster operator.
// It is called by KOTS admin console and will preposition images before creating a job to truly upgrade the cluster.
func UpgradeCmd() *cobra.Command {
	var installationFile, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string
	var installation *ecv1beta1.Installation

	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:          "upgrade",
		Short:        "create a job to upgrade the embedded cluster operator",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			installation, err = getInstallationFromFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to get installation from file: %w", err)
			}

			// set the runtime config from the installation spec
			rc.Set(installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logrus.New()
			logger.Info("Upgrade job creation started")

			cli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			logger.WithFields(logrus.Fields{
				"installation": installation.Name,
				"k0s_version":  installation.Spec.Config.Version,
			}).Info("Preparing upgrade")

			// create the installation object so that kotsadm can immediately find it and watch it for the upgrade process
			err = upgrade.CreateInstallation(cmd.Context(), cli, installation, logger)
			if err != nil {
				return fmt.Errorf("apply installation: %w", err)
			}
			previousInstallation, err := kubeutils.GetPreviousInstallation(cmd.Context(), cli, installation)
			if err != nil {
				return fmt.Errorf("get previous installation: %w", err)
			}

			err = upgrade.CreateUpgradeJob(
				cmd.Context(), cli, rc, installation,
				localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion,
				previousInstallation.Spec.Config.Version,
			)
			if err != nil {
				return fmt.Errorf("failed to upgrade: %w", err)
			}

			logger.Info("Upgrade job created successfully")

			return nil
		},
	}

	// TODO(upgrade): local-artifact-mirror-image should be included in the installation object
	cmd.Flags().StringVar(&localArtifactMirrorImage, "local-artifact-mirror-image", "", "Local artifact mirror image")
	err := cmd.MarkFlagRequired("local-artifact-mirror-image")
	if err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	err = cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}

	// For online upgrades
	cmd.Flags().StringVar(&licenseID, "license-id", "", "License ID for online upgrades")
	cmd.Flags().StringVar(&appSlug, "app-slug", "", "App slug for online upgrades")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "Channel ID for online upgrades")
	cmd.Flags().StringVar(&appVersion, "app-version", "", "App version for online upgrades")

	return cmd
}

func getInstallationFromFile(path string) (*ecv1beta1.Installation, error) {
	data, err := readInstallationFile(path)
	if err != nil {
		return nil, fmt.Errorf("read installation file: %w", err)
	}

	installation, err := decodeInstallation(data)
	if err != nil {
		return nil, fmt.Errorf("decode installation: %w", err)
	}

	return installation, nil
}

func readInstallationFile(path string) ([]byte, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return b, nil
	}
	b, err := helpers.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return b, nil
}

func decodeInstallation(data []byte) (*ecv1beta1.Installation, error) {
	scheme := runtime.NewScheme()
	err := ecv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add to scheme: %w", err)
	}

	decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	in, ok := obj.(*ecv1beta1.Installation)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}
	return in, nil
}
