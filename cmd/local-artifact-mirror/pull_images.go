package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// imagesCommand pulls images from the registry running in the cluster and stores
// them locally. This command is used during cluster upgrades when we want to fetch
// the most up to date images. Images are stored in a tarball file in the default
// location.
func PullImagesCmd(ctx context.Context, v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images <installation-name>",
		Short: "Pull images artifacts for an airgap installation",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			v.BindPFlag("data-dir", cmd.Flags().Lookup("data-dir"))

			if len(args) != 1 {
				return errors.New("expected installation name as argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			dataDir := v.GetString("data-dir")

			// Support for older env vars
			flag := cmd.Flags().Lookup("data-dir")
			if flag == nil || !flag.Changed {
				if os.Getenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR") != "" {
					dataDir = os.Getenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR")
				}
			}

			provider := defaults.NewProvider(dataDir)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			in, err := fetchAndValidateInstallation(ctx, args[0])
			if err != nil {
				return err
			}

			from := in.Spec.Artifacts.Images
			logrus.Infof("fetching images artifact from %s", from)
			location, err := pullArtifact(ctx, from)
			if err != nil {
				return fmt.Errorf("unable to fetch artifact: %w", err)
			}
			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				os.RemoveAll(location)
			}()

			dst := filepath.Join(provider.EmbeddedClusterImagesSubDir(), ImagesDstArtifactName)
			src := filepath.Join(location, ImagesSrcArtifactName)
			logrus.Infof("%s > %s", src, dst)
			if err := helpers.MoveFile(src, dst); err != nil {
				return fmt.Errorf("unable to move images bundle: %w", err)
			}

			logrus.Infof("images materialized under %s", dst)
			return nil
		},
	}

	cmd.Flags().String("data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.MarkFlagRequired("data-dir")

	return cmd
}
