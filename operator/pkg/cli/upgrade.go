package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// UpgradeCmd returns a cobra command for creating a job to upgrade the embedded cluster operator.
// It is called by KOTS admin console and will preposition images before creating a job to truly upgrade the cluster.
func UpgradeCmd() *cobra.Command {
	var installationFile, localArtifactMirrorImage string

	cmd := &cobra.Command{
		Use:          "upgrade",
		Short:        "create a job to upgrade the embedded cluster operator",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Upgrade job creation started")

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			installationData, err := readInstallationFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			in, err := decodeInstallation(cmd.Context(), []byte(installationData))
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(in.Spec.RuntimeConfig)

			fmt.Printf("Preparing upgrade to installation %s (k0s version %s)\n", in.Name, in.Spec.Config.Version)

			// create the installation object so that kotsadm can immediately find it and watch it for the upgrade process
			err = upgrade.CreateInstallation(cmd.Context(), cli, in)
			if err != nil {
				return fmt.Errorf("apply installation: %w", err)
			}
			previousInstallation, err := kubeutils.GetPreviousInstallation(cmd.Context(), cli, in)
			if err != nil {
				return fmt.Errorf("get previous installation: %w", err)
			}

			err = upgrade.CreateUpgradeJob(cmd.Context(), cli, in, localArtifactMirrorImage, previousInstallation.Spec.Config.Version)
			if err != nil {
				return fmt.Errorf("failed to upgrade: %w", err)
			}

			fmt.Println("Upgrade job created successfully")

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

func readInstallationFile(path string) ([]byte, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return b, nil
}

func decodeInstallation(ctx context.Context, data []byte) (*clusterv1beta1.Installation, error) {
	scheme := runtime.NewScheme()
	err := clusterv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add to scheme: %w", err)
	}

	decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	in, ok := obj.(*clusterv1beta1.Installation)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}
	return in, nil
}
