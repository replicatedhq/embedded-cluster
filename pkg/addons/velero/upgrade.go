package velero

import (
	"context"
	"log/slog"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *Velero) Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string) error {
	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		slog.Info("Release not found, installing", "release", releaseName, "namespace", namespace)
		if err := v.Install(ctx, kcli, hcli, overrides, nil); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := v.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    namespace,
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
