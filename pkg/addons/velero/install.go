package velero

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (v *Velero) Install(ctx context.Context, writer *spinner.MessageWriter, opts types.InstallOptions, overrides []string) error {
	if err := v.createPreRequisites(ctx, opts); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := v.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    v.ChartLocation(opts.Domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    v.Namespace(),
	}

	if opts.IsDryRun {
		manifests, err := v.hcli.Render(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		v.dryRunManifests = append(v.dryRunManifests, manifests...)
	} else {
		_, err = v.hcli.Install(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}

func (v *Velero) createPreRequisites(ctx context.Context, opts types.InstallOptions) error {
	if err := v.createNamespace(ctx, opts); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := v.createCredentialsSecret(ctx, opts); err != nil {
		return errors.Wrap(err, "create credentials secret")
	}

	return nil
}

func (v *Velero) createNamespace(ctx context.Context, opts types.InstallOptions) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v.Namespace(),
		},
	}
	if opts.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := v.kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (v *Velero) createCredentialsSecret(ctx context.Context, opts types.InstallOptions) error {
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
	if opts.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		v.dryRunManifests = append(v.dryRunManifests, b.Bytes())
	} else {
		if err := v.kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
