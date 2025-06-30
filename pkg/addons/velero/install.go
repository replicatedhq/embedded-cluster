package velero

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *Velero) Install(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	if err := v.createPreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := v.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	opts := helm.InstallOptions{
		ReleaseName:  v.ReleaseName(),
		ChartPath:    v.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    v.Namespace(),
	}

	if v.DryRun {
		manifests, err := hcli.Render(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		v.dryRunManifests = append(v.dryRunManifests, manifests...)
	} else {
		_, err = hcli.Install(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}

func (v *Velero) createPreRequisites(ctx context.Context, kcli client.Client) error {
	if err := v.createNamespace(ctx, kcli); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := v.createCredentialsSecret(ctx, kcli); err != nil {
		return errors.Wrap(err, "create credentials secret")
	}

	return nil
}

func (v *Velero) createNamespace(ctx context.Context, kcli client.Client) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v.Namespace(),
		},
	}
	if v.DryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (v *Velero) createCredentialsSecret(ctx context.Context, kcli client.Client) error {
	obj := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      _credentialsSecretName,
			Namespace: v.Namespace(),
		},
		Type: "Opaque",
	}
	if v.DryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
