package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *EmbeddedClusterOperator) Install(ctx context.Context, kcli client.Client, hcli *helm.Helm, writer *spinner.MessageWriter) error {
	if err := o.prepare(); err != nil {
		return errors.Wrap(err, "prepare metrics operator")
	}

	_, err := hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
	})
	if err != nil {
		return errors.Wrap(err, "install metrics operator")
	}

	return nil
}

func (o *EmbeddedClusterOperator) InstallForRestore(ctx context.Context, kcli client.Client, hcli *helm.Helm, writer *spinner.MessageWriter) error {
	// not included in a restore
	return nil
}
