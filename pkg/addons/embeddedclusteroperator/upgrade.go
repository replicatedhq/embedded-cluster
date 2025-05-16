package embeddedclusteroperator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *EmbeddedClusterOperator) Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string) error {
	err := UpgradeEnsureCAConfigmap(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "ensure CA configmap")
	}

	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, namespace)
		if err := e.Install(ctx, kcli, hcli, overrides, nil); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	values, err := e.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
		ForceUpgrade: false,
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

// UpgradeEnsureCAConfigmap ensures that the CA configmap exists in the embedded-cluster namespace.
// This is needed for the copy artifacts job and did not exist in the previous versions of
// Embedded Cluster.
func UpgradeEnsureCAConfigmap(ctx context.Context, kcli client.Client) error {
	var cm corev1.ConfigMap
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: runtimeconfig.KotsadmNamespace,
		Name:      "kotsadm-private-cas",
	}, &cm)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return errors.Wrap(err, "get kotsadm-private-cas configmap")
	}

	return ensureCAConfigmap(ctx, kcli, cm.Data)
}
