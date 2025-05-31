package openebs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func (o *OpenEBS) Install(ctx context.Context, writer *spinner.MessageWriter, opts types.InstallOptions, overrides []string) error {
	values, err := o.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    o.ChartLocation(opts.Domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
	}

	_, err = o.hcli.Install(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}
