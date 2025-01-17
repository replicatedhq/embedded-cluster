package adminconsole

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *AdminConsole) Upgrade(ctx context.Context, kcli client.Client) error {
	if err := a.prepare(); err != nil {
		return errors.Wrap(err, "prepare admin console")
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		KubeConfig: runtimeconfig.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "upgrade admin console")
	}

	return nil
}
