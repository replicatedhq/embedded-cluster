package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func (e *EmbeddedClusterOperator) Install(ctx context.Context, writer *spinner.MessageWriter, opts types.InstallOptions, overrides []string) error {
	values, err := e.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(opts.Domains),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    e.Namespace(),
		Labels:       getBackupLabels(),
	}

	if opts.IsDryRun {
		manifests, err := e.hcli.Render(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		e.dryRunManifests = append(e.dryRunManifests, manifests...)
	} else {
		_, err = e.hcli.Install(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}
