package embeddedclusteroperator

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

func (e *EmbeddedClusterOperator) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	exists, err := hcli.ReleaseExists(ctx, e.Namespace(), e.ReleaseName())
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", e.ReleaseName(), e.Namespace())
		if err := e.Install(ctx, logf, kcli, mcli, hcli, domains, overrides); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := e.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  e.ReleaseName(),
		ChartPath:    e.ChartLocation(domains),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    e.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
		// CRDs are bootstrapped by kubeutils.EnsureInstallationCRD before this runs, which claims SSA
		// field ownership under "embedded-cluster". Server-side apply on the chart's bundled CRDs
		// then conflicts on .metadata.annotations.controller-gen.kubebuilder.io/version. Disabling
		// SSA falls back to client-side apply, matching the Helm 3 behavior this chart was designed against.
		DisableSSA: true,
		LogFn:      helm.LogFn(logf),
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
