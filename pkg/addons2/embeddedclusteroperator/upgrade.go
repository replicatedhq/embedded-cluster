package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *EmbeddedClusterOperator) Upgrade(ctx context.Context, kcli client.Client, hcli *helm.Helm) error {
	if err := o.prepare(); err != nil {
		return errors.Wrap(err, "prepare embedded cluster operator")
	}

	_, err := hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
		Force:        true,
	})
	if err != nil {
		return errors.Wrap(err, "upgrade metrics operator")
	}

	return nil
}
