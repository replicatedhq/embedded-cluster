package openebs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
)

func (o *OpenEBS) Upgrade(ctx context.Context, opts types.InstallOptions, overrides []string) error {
	exists, err := o.hcli.ReleaseExists(ctx, o.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, o.Namespace())
		if err := o.Install(ctx, nil, opts, overrides); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := o.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    o.ChartLocation(opts.Domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
		Force:        false,
	}

	_, err = o.hcli.Upgrade(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
