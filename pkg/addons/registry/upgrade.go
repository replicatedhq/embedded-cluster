package registry

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// s3SecretName is the name of the Registry s3 secret.
	// This secret name is defined in the chart in the release metadata.
	s3SecretName = "seaweedfs-s3-rw"
)

// Upgrade upgrades the registry chart to the latest version.
func (r *Registry) Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string) error {
	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, namespace)
		if err := r.Install(ctx, kcli, hcli, overrides, nil); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	if err := r.createUpgradePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := r.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

func (r *Registry) createUpgradePreRequisites(ctx context.Context, kcli client.Client) error {
	if r.IsHA {
		if err := ensureS3Secret(ctx, kcli); err != nil {
			return errors.Wrap(err, "create s3 secret")
		}
	}

	return nil
}

func ensureS3Secret(ctx context.Context, kcli client.Client) error {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get seaweedfs s3 rw creds")
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: s3SecretName, Namespace: namespace},
		Data: map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		},
	}

	obj.ObjectMeta.Labels = seaweedfs.ApplyLabels(obj.ObjectMeta.Labels, "s3")

	if err := kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "create s3 secret")
	}
	return nil
}
