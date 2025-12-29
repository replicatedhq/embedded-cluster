package cli

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg-new/artifacts"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// DistributeArtifactsCmd returns a cobra command for distributing artifacts to all nodes.
// It is called by KOTS during app-only upgrades to ensure artifacts are available for joining nodes.
func DistributeArtifactsCmd() *cobra.Command {
	var inFile, licenseID, appSlug, channelID, appVersion string

	cmd := &cobra.Command{
		Use:          "distribute-artifacts",
		Short:        "Distribute artifacts to all nodes for the given installation",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logrus.New()
			logger.Info("Starting artifact distribution")

			in, err := getInstallationFromFile(inFile)
			if err != nil {
				return fmt.Errorf("get installation: %w", err)
			}

			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("create kube client: %w", err)
			}

			rc := runtimeconfig.New(in.Spec.RuntimeConfig)

			// Get metadata for local artifact mirror image
			meta, err := release.MetadataFor(cmd.Context(), in, kcli)
			if err != nil {
				return fmt.Errorf("get release metadata: %w", err)
			}

			localArtifactMirrorImage, ok := meta.Artifacts["local-artifact-mirror-image"]
			if !ok || localArtifactMirrorImage == "" {
				return fmt.Errorf("local artifact mirror image not found in release metadata")
			}

			// Use config version if appVersion not provided
			if appVersion == "" && in.Spec.Config != nil {
				appVersion = in.Spec.Config.Version
			}

			// Distribute artifacts to all nodes (for airgap, this also creates and waits for the autopilot plan)
			logger.Info("Distributing artifacts to all nodes...")
			if err := artifacts.DistributeArtifacts(
				cmd.Context(), kcli, rc, in,
				localArtifactMirrorImage,
				licenseID,
				appSlug,
				channelID,
				appVersion,
			); err != nil {
				return fmt.Errorf("distribute artifacts: %w", err)
			}

			logger.Info("Artifacts distributed successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&inFile, "installation", "", "Path to installation file (use '-' for stdin)")
	cmd.Flags().StringVar(&licenseID, "license-id", "", "License ID")
	cmd.Flags().StringVar(&appSlug, "app-slug", "", "App slug")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "Channel ID")
	cmd.Flags().StringVar(&appVersion, "app-version", "", "App version (defaults to installation config version)")

	if err := cmd.MarkFlagRequired("installation"); err != nil {
		panic(err)
	}

	return cmd
}
