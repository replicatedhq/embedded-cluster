package registry

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// seaweedfsS3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the values-ha.yaml file in the release metadata.
	seaweedfsS3SecretName = "seaweedfs-s3-rw"
)

// Upgrade upgrades the registry chart to the latest version.
func (r *Registry) Upgrade(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec, overrides []string) error {
	exists, err := clients.HelmClient.ReleaseExists(ctx, r.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, r.Namespace())
		if err := r.Install(ctx, clients, nil, inSpec, overrides, types.InstallOptions{}); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	if err := r.createUpgradePreRequisites(ctx, clients, inSpec); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := r.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	// only add tls secret value if the secret exists
	// this is for backwards compatibility when the registry was deployed without TLS
	var secret corev1.Secret
	if err := clients.K8sClient.Get(ctx, client.ObjectKey{Namespace: r.Namespace(), Name: tlsSecretName}, &secret); err == nil {
		values["tlsSecretName"] = tlsSecretName
	} else if !k8serrors.IsNotFound(err) {
		return errors.Wrap(err, "get tls secret")
	}

	helmOpts := helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    r.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    r.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
	}

	_, err = clients.HelmClient.Upgrade(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

func (r *Registry) createUpgradePreRequisites(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec) error {
	if inSpec.HighAvailability {
		if err := ensureS3Secret(ctx, clients); err != nil {
			return errors.Wrap(err, "create s3 secret")
		}
	}

	return nil
}

func ensureS3Secret(ctx context.Context, clients types.Clients) error {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, clients.K8sClient)
	if err != nil {
		return errors.Wrap(err, "get seaweedfs s3 rw creds")
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SecretName, Namespace: namespace},
		Data: map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		},
	}

	obj.ObjectMeta.Labels = seaweedfs.ApplyLabels(obj.ObjectMeta.Labels, "s3")

	if err := clients.K8sClient.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "create s3 secret")
	}
	return nil
}
