package velero

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *Velero) Install(ctx context.Context, kcli client.Client, writer *spinner.MessageWriter) error {
	if err := v.prepare(); err != nil {
		return errors.Wrap(err, "prepare velero")
	}

	if err := v.createPreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
	})
	if err != nil {
		return errors.Wrap(err, "install velero")
	}

	return nil
}

func (v *Velero) createPreRequisites(ctx context.Context, kcli client.Client) error {
	if err := createNamespace(ctx, kcli, namespace); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := createCredentialsSecret(ctx, kcli); err != nil {
		return errors.Wrap(err, "create credentials secret")
	}

	return nil
}

func createNamespace(ctx context.Context, kcli client.Client, namespace string) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := kcli.Create(ctx, &ns); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func createCredentialsSecret(ctx context.Context, kcli client.Client) error {
	credentialsSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialsSecretName,
			Namespace: namespace,
		},
		Type: "Opaque",
	}
	if err := kcli.Create(ctx, &credentialsSecret); err != nil {
		return errors.Wrap(err, "create credentials secret")
	}

	return nil
}
