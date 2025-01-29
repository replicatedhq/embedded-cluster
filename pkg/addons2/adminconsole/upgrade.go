package adminconsole

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *AdminConsole) Upgrade(ctx context.Context, kcli client.Client, hcli *helm.Helm, overrides []string) error {
	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		// admin console must exist during upgrade
		return errors.New("admin console release not found")
	}

	if err := a.prepare(overrides); err != nil {
		return errors.Wrap(err, "prepare admin console")
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
