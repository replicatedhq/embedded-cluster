package openebs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Install(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	rc runtimeconfig.RuntimeConfig, domains ecv1beta1.Domains,
	overrides []string, writer *spinner.MessageWriter,
) error {
	values, err := o.GenerateHelmValues(ctx, kcli, rc, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  o.ReleaseName(),
		ChartPath:    o.ChartLocation(ecv1beta1.Domains{}),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}
