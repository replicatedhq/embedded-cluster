package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// imagesCommand pulls images from the registry running in the cluster and stores
// them locally. This command is used during cluster upgrades when we want to fetch
// the most up to date images. Images are stored in a tarball file in the default
// location.
func PullImagesCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images INSTALLATION",
		Short: "Pull images artifacts for an airgap installation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			kcli, err := cli.KCLIGetter()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}

			in, err := fetchAndValidateInstallation(ctx, kcli, args[0])
			if err != nil {
				return err
			}

			if !in.Spec.AirGap {
				return fmt.Errorf("pulling images is not supported for online installations")
			}

			from := in.Spec.Artifacts.Images
			logrus.Infof("fetching images artifact from %s", from)
			location, err := cli.PullArtifact(ctx, kcli, from)
			if err != nil {
				return fmt.Errorf("unable to fetch artifact: %w", err)
			}
			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				os.RemoveAll(location)
			}()

			dst := filepath.Join(cli.RC.EmbeddedClusterImagesSubDir(), ImagesDstArtifactName)
			src := filepath.Join(location, ImagesSrcArtifactName)
			logrus.Infof("%s > %s", src, dst)
			if err := helpers.MoveFile(src, dst); err != nil {
				return fmt.Errorf("unable to move images bundle: %w", err)
			}

			logrus.Infof("images materialized under %s", dst)
			return nil
		},
	}

	return cmd
}
