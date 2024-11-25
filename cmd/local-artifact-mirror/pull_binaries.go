package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// binariesCommands pulls the binary artifact from the registry running in the cluster and stores
// it locally. This command is used during cluster upgrades when we want to fetch the most up to
// date binaries. The binaries are stored in the /usr/local/bin directory and they overwrite the
// existing binaries.
func PullBinariesCmd(ctx context.Context, v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binaries <installation-name>",
		Short: "Pull binaries artifacts for an airgap installation",
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

			runtimeconfig.SetDataDir(dataDir)
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			in, err := fetchAndValidateInstallation(ctx, args[0])
			if err != nil {
				return err
			}

			from := in.Spec.Artifacts.EmbeddedClusterBinary
			logrus.Infof("fetching embedded cluster binary artifact from %s", from)
			location, err := pullArtifact(ctx, from)
			if err != nil {
				return fmt.Errorf("unable to fetch artifact: %w", err)
			}
			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				os.RemoveAll(location)
			}()
			bin := filepath.Join(location, EmbeddedClusterBinaryArtifactName)
			namedBin := filepath.Join(location, in.Spec.BinaryName)
			if err := os.Rename(bin, namedBin); err != nil {
				return fmt.Errorf("unable to rename binary: %w", err)
			}

			if err := os.Chmod(namedBin, 0755); err != nil {
				return fmt.Errorf("unable to change permissions on %s: %w", bin, err)
			}

			out := bytes.NewBuffer(nil)

			materializeCmdArgs := []string{"materialize", "--data-dir", runtimeconfig.EmbeddedClusterHomeDirectory()}
			materializeCmd := exec.Command(namedBin, materializeCmdArgs...)
			materializeCmd.Stdout = out
			materializeCmd.Stderr = out

			logrus.Infof("running command: %s with args: %v", namedBin, materializeCmdArgs)
			if err := materializeCmd.Run(); err != nil {
				logrus.Error(out.String())
				return err
			}

			logrus.Infof("embedded cluster binaries materialized")

			return nil
		},
	}

	cmd.Flags().String("data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.MarkFlagRequired("data-dir")

	return cmd
}
