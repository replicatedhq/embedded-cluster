package velero

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
)

func (v *Velero) Upgrade(ctx context.Context, opts types.InstallOptions, overrides []string) error {
	exists, err := v.hcli.ReleaseExists(ctx, v.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, v.Namespace())
		if err := v.Install(ctx, nil, opts, overrides); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := v.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    v.ChartLocation(opts.Domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    v.Namespace(),
		Force:        false,
	}

	_, err = v.hcli.Upgrade(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
