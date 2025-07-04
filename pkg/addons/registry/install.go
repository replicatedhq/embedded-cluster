package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/certs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Registry) Install(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains,
	overrides []string,
) error {
	registryIP, err := GetRegistryClusterIP(r.ServiceCIDR)
	if err != nil {
		return errors.Wrap(err, "get registry cluster IP")
	}

	if err := r.createPreRequisites(ctx, kcli, registryIP); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := r.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  r.ReleaseName(),
		ChartPath:    r.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    r.Namespace(),
		Labels:       getBackupLabels(),
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}

func (r *Registry) createPreRequisites(ctx context.Context, kcli client.Client, registryIP string) error {
	if err := r.createNamespace(ctx, kcli); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := r.createAuthSecret(ctx, kcli); err != nil {
		return errors.Wrap(err, "create registry-auth secret")
	}

	if err := r.createTLSSecret(ctx, kcli, registryIP); err != nil {
		return errors.Wrap(err, "create registry tls secret")
	}

	return nil
}

func (r *Registry) createNamespace(ctx context.Context, kcli client.Client) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Namespace(),
		},
	}
	if err := kcli.Create(ctx, &ns); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (r *Registry) createAuthSecret(ctx context.Context, kcli client.Client) error {
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
	if err := kcli.Create(ctx, &htpasswd); err != nil {
		return err
	}

	return nil
}

func (r *Registry) createTLSSecret(ctx context.Context, kcli client.Client, registryIP string) error {
	tlsCert, tlsKey, err := r.generateRegistryTLS(registryIP)
	if err != nil {
		return errors.Wrap(err, "generate registry tls")
	}

	tlsSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      _tlsSecretName,
			Namespace: r.Namespace(),
			Labels: map[string]string{
				"app": "docker-registry", // this is the backup/restore label for the registry component
			},
		},
		StringData: map[string]string{"tls.crt": tlsCert, "tls.key": tlsKey},
		Type:       "Opaque",
	}
	if err := kcli.Create(ctx, tlsSecret); err != nil {
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
		return "", "", fmt.Errorf("failed to create cert builder: %w", err)
	}
	return builder.Generate()
}
