package openebs

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *OpenEBS) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	err := o.ensurePreUpgradeHooksDeleted(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "ensure pre-upgrade hooks deleted")
	}

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
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

// ensurePostUpgradeHooksDeleted will delete helm hooks if for some reason they fail. It is
// necessary if the hook does not have the "before-hook-creation" delete policy and instead has the
// policy "hook-succeeded".
func (o *OpenEBS) ensurePreUpgradeHooksDeleted(ctx context.Context, kcli client.Client) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.Namespace(),
			Name:      fmt.Sprintf("%s-pre-upgrade-hook", o.ReleaseName()),
		},
	}
	err := kcli.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete openebs-pre-upgrade-hook job")
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-pre-upgrade-hook", o.ReleaseName()),
		},
	}
	err = kcli.Delete(ctx, crb)
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete openebs-pre-upgrade-hook cluster role binding")
	}

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-pre-upgrade-hook", o.ReleaseName()),
		},
	}
	err = kcli.Delete(ctx, cr)
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete openebs-pre-upgrade-hook cluster role")
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.Namespace(),
			Name:      fmt.Sprintf("%s-pre-upgrade-hook", o.ReleaseName()),
		},
	}
	err = kcli.Delete(ctx, sa)
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "delete openebs-pre-upgrade-hook service account")
	}

	return nil
}
