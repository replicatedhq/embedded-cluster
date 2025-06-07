package openebs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func (o *OpenEBS) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	values, err := o.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    o.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
	}

	_, err = clients.HelmClient.Install(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}
