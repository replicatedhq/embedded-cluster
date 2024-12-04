package openebs

import (
	"context"
	"fmt"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

func (o *OpenEBS) Install(ctx context.Context, clusterConfig *k0sconfig.ClusterConfig) error {
	helm, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return fmt.Errorf("unable to create helm client: %w", err)
	}

	release, err := helm.Install(ctx, releaseName, Metadata.Location, Metadata.Version, helmValues, namespace)
	if err != nil {
		return fmt.Errorf("unable to install openebs: %w", err)
	}

	fmt.Printf("OpenEBS installed successfully: %s\n", release.Manifest)

	return nil
}
