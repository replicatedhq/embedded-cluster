package adminconsole

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *AdminConsole) Upgrade(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec, overrides []string) error {
	exists, err := clients.HelmClient.ReleaseExists(ctx, a.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		// admin console must exist during upgrade
		return errors.New("admin console release not found")
	}

	values, err := a.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	err = ensurePostUpgradeHooksDeleted(ctx, clients, a.Namespace())
	if err != nil {
		return errors.Wrap(err, "ensure hooks deleted")
	}

	helmOpts := helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    a.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    a.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
	}

	_, err = clients.HelmClient.Upgrade(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

// ensurePostUpgradeHooksDeleted will delete helm hooks if for some reason they fail. It is
// necessary if the hook does not have the "before-hook-creation" delete policy.
func ensurePostUpgradeHooksDeleted(ctx context.Context, clients types.Clients, namespace string) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "kotsadm-keep-resources",
		},
	}
	err := clients.K8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete kotsadm-keep-resources job")
	}

	return nil
}
