package openebs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Install(ctx context.Context, kcli client.Client, hcli *helm.Helm, writer *spinner.MessageWriter) error {
	if err := o.prepare(); err != nil {
		return errors.Wrap(err, "prepare openebs")
	}

	_, err := hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
	})
	if err != nil {
		return errors.Wrap(err, "install openebs")
	}

	return nil
}

func (o *OpenEBS) InstallForRestore(ctx context.Context, kcli client.Client, hcli *helm.Helm, writer *spinner.MessageWriter) error {
	return o.Install(ctx, kcli, hcli, writer)
}
