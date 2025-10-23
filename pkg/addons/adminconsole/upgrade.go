package adminconsole

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *AdminConsole) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	exists, err := hcli.ReleaseExists(ctx, a.Namespace(), a.ReleaseName())
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		// admin console must exist during upgrade
		return errors.New("admin console release not found")
	}

	values, err := a.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	err = a.ensurePostUpgradeHooksDeleted(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "ensure hooks deleted")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  a.ReleaseName(),
		ChartPath:    a.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    a.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
		LogFn:        helm.LogFn(logf),
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

// ensurePostUpgradeHooksDeleted will delete helm hooks if for some reason they fail. It is
// necessary if the hook does not have the "before-hook-creation" delete policy.
func (a *AdminConsole) ensurePostUpgradeHooksDeleted(ctx context.Context, kcli client.Client) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.Namespace(),
			Name:      "kotsadm-keep-resources",
		},
	}
	err := kcli.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete kotsadm-keep-resources job")
	}

	return nil
}
