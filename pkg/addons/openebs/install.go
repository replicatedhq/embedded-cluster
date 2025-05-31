package openebs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Install(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, overrides []string, writer *spinner.MessageWriter) error {
	values, err := o.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    o.ChartLocation(),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    namespace,
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}
