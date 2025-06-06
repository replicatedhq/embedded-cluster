package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/certs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Registry) Install(ctx context.Context, writer *spinner.MessageWriter, opts types.InstallOptions, overrides []string) error {
	registryIP, err := GetRegistryClusterIP(opts.ServiceCIDR)
	if err != nil {
		return errors.Wrap(err, "get registry cluster IP")
	}

	if err := r.createPreRequisites(ctx, opts, registryIP); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := r.GenerateHelmValues(ctx, opts, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    r.ChartLocation(opts.Domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    r.Namespace(),
		Labels:       getBackupLabels(),
	}

	_, err = r.hcli.Install(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}

func (r *Registry) createPreRequisites(ctx context.Context, opts types.InstallOptions, registryIP string) error {
	if err := r.createNamespace(ctx); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := r.createAuthSecret(ctx); err != nil {
		return errors.Wrap(err, "create registry-auth secret")
	}

	if err := r.createTLSSecret(ctx, registryIP); err != nil {
		return errors.Wrap(err, "create registry tls secret")
	}

	return nil
}

func (r *Registry) createNamespace(ctx context.Context) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Namespace(),
		},
	}
	if err := r.kcli.Create(ctx, &ns); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (r *Registry) createAuthSecret(ctx context.Context) error {
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(registryPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "hash registry password")
	}

	htpasswd := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-auth",
			Namespace: r.Namespace(),
			Labels: map[string]string{
				"app": "docker-registry", // this is the backup/restore label for the registry component
			},
		},
		StringData: map[string]string{
			"htpasswd": fmt.Sprintf("embedded-cluster:%s", string(hashPassword)),
		},
		Type: "Opaque",
	}
	if err := r.kcli.Create(ctx, &htpasswd); err != nil {
		return err
	}

	return nil
}

func (r *Registry) createTLSSecret(ctx context.Context, registryIP string) error {
	tlsCert, tlsKey, err := r.generateRegistryTLS(registryIP)
	if err != nil {
		return errors.Wrap(err, "generate registry tls")
	}

	tlsSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: r.Namespace(),
			Labels: map[string]string{
				"app": "docker-registry", // this is the backup/restore label for the registry component
			},
		},
		StringData: map[string]string{"tls.crt": tlsCert, "tls.key": tlsKey},
		Type:       "Opaque",
	}
	if err := r.kcli.Create(ctx, tlsSecret); err != nil {
		return errors.Wrap(err, "create registry tls secret")
	}

	return nil
}

func (r *Registry) generateRegistryTLS(registryIP string) (string, string, error) {
	opts := []certs.Option{
		certs.WithCommonName("registry"),
		certs.WithDuration(365 * 24 * time.Hour),
		certs.WithIPAddress(registryIP),
	}

	for _, name := range []string{
		"registry",
		fmt.Sprintf("registry.%s.svc", r.Namespace()),
		fmt.Sprintf("registry.%s.svc.cluster.local", r.Namespace()),
	} {
		opts = append(opts, certs.WithDNSName(name))
	}

	builder, err := certs.NewBuilder(opts...)
	if err != nil {
		return "", "", errors.Wrap(err, "create cert builder")
	}
	return builder.Generate()
}
