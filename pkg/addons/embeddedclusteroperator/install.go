package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func (e *EmbeddedClusterOperator) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	values, err := e.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    e.Namespace(),
		Labels:       getBackupLabels(),
	}

	if clients.IsDryRun {
		manifests, err := clients.HelmClient.Render(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		e.dryRunManifests = append(e.dryRunManifests, manifests...)
	} else {
		_, err = clients.HelmClient.Install(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}
