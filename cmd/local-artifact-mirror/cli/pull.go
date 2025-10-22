package cli

import (
	"context"
	"encoding/base64"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These constant define the expected names of the files in the registry.
const (
	EmbeddedClusterBinaryArtifactName = "embedded-cluster-amd64"
	ImagesSrcArtifactName             = "images-amd64.tar"
	ImagesDstArtifactName             = "ec-images-amd64.tar"
	HelmChartsArtifactName            = "charts.tar.gz"
)

func PullCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull artifacts for an airgap installation",
	}

	cmd.AddCommand(PullBinariesCmd(cli))
	cmd.AddCommand(PullImagesCmd(cli))
	cmd.AddCommand(PullHelmChartsCmd(cli))

	return cmd
}

// fetchAndValidateInstallation fetches an Installation object from its name or directly decodes it
// and checks if it is valid for an airgap cluster deployment.
func fetchAndValidateInstallation(ctx context.Context, kcli client.Client, iname string) (*ecv1beta1.Installation, error) {
	in, err := decodeInstallation(ctx, iname)
	if err != nil {
		in, err = kubeutils.GetInstallation(ctx, kcli, iname)
		if err != nil {
			return nil, err
		}
	}

	if in.Spec.AirGap && in.Spec.Artifacts == nil {
		return nil, fmt.Errorf("airgap installation has no artifacts")
	}

	return in, nil
}

// decodeInstallation decodes an Installation object from a string.
func decodeInstallation(ctx context.Context, data string) (*ecv1beta1.Installation, error) {
	logrus.Info("decoding installation")

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	scheme := runtime.NewScheme()
	err = ecv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add to scheme: %w", err)
	}

	decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode
	obj, _, err := decode(decoded, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	in, ok := obj.(*ecv1beta1.Installation)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	return in, nil
}
