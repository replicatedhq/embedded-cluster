package velero

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *Velero) Upgrade(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, overrides []string) error {
	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, namespace)
		if err := v.Install(ctx, logf, kcli, mcli, hcli, overrides, nil); err != nil {
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
		ChartPath:    v.ChartLocation(),
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
