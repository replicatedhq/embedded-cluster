package registry

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// seaweedfsS3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the values-ha.yaml file in the release metadata.
	seaweedfsS3SecretName = "seaweedfs-s3-rw"
)

// Upgrade upgrades the registry chart to the latest version.
func (r *Registry) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	exists, err := hcli.ReleaseExists(ctx, r.Namespace(), r.ReleaseName())
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", r.ReleaseName(), r.Namespace())
		if err := r.Install(ctx, logf, kcli, mcli, hcli, domains, overrides); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	}

	if err := r.createUpgradePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := r.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  r.ReleaseName(),
		ChartPath:    r.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    r.Namespace(),
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
		if err := r.ensureS3Secret(ctx, kcli); err != nil {
			return errors.Wrap(err, "create s3 secret")
		}
	}

	return nil
}

func (r *Registry) ensureS3Secret(ctx context.Context, kcli client.Client) error {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get seaweedfs s3 rw creds")
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SecretName, Namespace: r.Namespace()},
		Data: map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		},
	}

	obj.Labels = seaweedfs.ApplyLabels(obj.Labels, "s3")

	if err := kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "create s3 secret")
	}
	return nil
}
