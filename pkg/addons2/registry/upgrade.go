package registry

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
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

func (r *Registry) Upgrade(ctx context.Context, kcli client.Client) error {
	if err := r.prepare(); err != nil {
		return errors.Wrap(err, "prepare registry")
	}

	if err := r.createUpgradePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		KubeConfig: runtimeconfig.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	var values map[string]interface{}
	if r.IsHA {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    namespace,
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "upgrade registry")
	}

	return nil
}

func (r *Registry) createUpgradePreRequisites(ctx context.Context, kcli client.Client) error {
	if err := createS3Secret(ctx, kcli); err != nil {
		return errors.Wrap(err, "create s3 secret")
	}

	return nil
}

func createS3Secret(ctx context.Context, kcli client.Client) error {
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
