package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

// kubecli holds a global reference to a Kubernetes client.
var kubecli client.Client

func PullCmd(ctx context.Context, v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull artifacts for an airgap installation",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			kc, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}
			kubecli = kc

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			os.Exit(1)
			return nil
		},
	}

	cmd.AddCommand(PullBinariesCmd(ctx, v))
	cmd.AddCommand(PullImagesCmd(ctx, v))
	cmd.AddCommand(PullHelmChartsCmd(ctx, v))

	return cmd
}

// fetchAndValidateInstallation fetches an Installation object from its name or directly decodes it
// and checks if it is valid for an airgap cluster deployment.
func fetchAndValidateInstallation(ctx context.Context, iname string) (*ecv1beta1.Installation, error) {
	in, err := decodeInstallation(ctx, iname)
	if err != nil {
		in, err = kubeutils.GetInstallation(ctx, kubecli, iname)
		if err != nil {
			return nil, err
		}
	}

	if !in.Spec.AirGap {
		return nil, fmt.Errorf("installation is not airgapped")
	} else if in.Spec.Artifacts == nil {
		return nil, fmt.Errorf("installation has no artifacts")
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
