package openebs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

func (o *OpenEBS) Upgrade(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec, overrides []string) error {
	exists, err := clients.HelmClient.ReleaseExists(ctx, o.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, o.Namespace())
		if err := o.Install(ctx, clients, nil, inSpec, overrides, types.InstallOptions{}); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := o.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    o.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
		Force:        false,
	}

	_, err = clients.HelmClient.Upgrade(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
