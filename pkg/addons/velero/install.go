package velero

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (v *Velero) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	if err := v.createPreRequisites(ctx, clients); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := v.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    v.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    v.Namespace(),
	}

	if clients.IsDryRun {
		manifests, err := clients.HelmClient.Render(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		v.dryRunManifests = append(v.dryRunManifests, manifests...)
	} else {
		_, err = clients.HelmClient.Install(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}

func (v *Velero) createPreRequisites(ctx context.Context, clients types.Clients) error {
	if err := v.createNamespace(ctx, clients); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := v.createCredentialsSecret(ctx, clients); err != nil {
		return errors.Wrap(err, "create credentials secret")
	}

	return nil
}

func (v *Velero) createNamespace(ctx context.Context, clients types.Clients) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v.Namespace(),
		},
	}
	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := clients.K8sClient.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (v *Velero) createCredentialsSecret(ctx context.Context, clients types.Clients) error {
	obj := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialsSecretName,
			Namespace: v.Namespace(),
		},
		Type: "Opaque",
	}
	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := clients.K8sClient.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
