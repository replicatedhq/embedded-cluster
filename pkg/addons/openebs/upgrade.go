package openebs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	exists, err := hcli.ReleaseExists(ctx, o.Namespace(), o.ReleaseName())
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", o.ReleaseName(), o.Namespace())
		if err := o.Install(ctx, logf, kcli, mcli, hcli, domains, overrides); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := o.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  o.ReleaseName(),
		ChartPath:    o.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    o.Namespace(),
		Force:        false,
		LogFn:        helm.LogFn(logf),
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
