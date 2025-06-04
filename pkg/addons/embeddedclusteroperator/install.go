package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *EmbeddedClusterOperator) Install(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, overrides []string, writer *spinner.MessageWriter) error {
	values, err := e.GenerateHelmValues(ctx, kcli, rc, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	opts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
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
