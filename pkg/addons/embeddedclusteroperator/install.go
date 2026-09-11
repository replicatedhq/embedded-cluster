package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *EmbeddedClusterOperator) Install(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	values, err := e.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	opts := helm.InstallOptions{
		ReleaseName:  e.ReleaseName(),
		ChartPath:    e.ChartLocation(domains),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    e.Namespace(),
		Labels:       getBackupLabels(),
		// CRDs are bootstrapped by kubeutils.EnsureInstallationCRD before this runs, which claims SSA
		// field ownership under "embedded-cluster" and conflicts with Helm 4's default server-side apply
		// on .metadata.annotations.controller-gen.kubebuilder.io/version. Mirrors the upgrade path.
		DisableSSA: true,
		LogFn:      helm.LogFn(logf),
	}

	if e.DryRun {
		manifests, err := hcli.Render(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		e.dryRunManifests = append(e.dryRunManifests, manifests...)
	} else {
		_, err = hcli.Install(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}
