package openebs

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Install(ctx context.Context, kcli client.Client, writer *spinner.MessageWriter) error {
	helm, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}

	_, err = helm.Install(ctx, releaseName, Metadata.Location, Metadata.Version, helmValues, namespace)
	if err != nil {
		return fmt.Errorf("install openebs: %w", err)
	}

	return nil
}
