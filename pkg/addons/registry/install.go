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
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Registry) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	var serviceCIDR string
	if inSpec.Network != nil && inSpec.Network.ServiceCIDR != "" {
		serviceCIDR = inSpec.Network.ServiceCIDR
	} else {
		var err error
		_, serviceCIDR, err = netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
		if err != nil {
			return fmt.Errorf("split default network CIDR: %w", err)
		}
	}

	registryIP, err := GetRegistryClusterIP(serviceCIDR)
	if err != nil {
		return errors.Wrap(err, "get registry cluster IP")
	}

	if err := r.createPreRequisites(ctx, clients, registryIP); err != nil {
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

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    r.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    r.Namespace(),
		Labels:       getBackupLabels(),
	}

	_, err = clients.HelmClient.Install(ctx, helmOpts)
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}

func (r *Registry) createPreRequisites(ctx context.Context, clients types.Clients, registryIP string) error {
	if err := r.createNamespace(ctx, clients); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := r.createAuthSecret(ctx, clients); err != nil {
		return errors.Wrap(err, "create registry-auth secret")
	}

	if err := r.createTLSSecret(ctx, clients, registryIP); err != nil {
		return errors.Wrap(err, "create registry tls secret")
	}

	return nil
}

func (r *Registry) createNamespace(ctx context.Context, clients types.Clients) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Namespace(),
		},
	}
	if err := clients.K8sClient.Create(ctx, &ns); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (r *Registry) createAuthSecret(ctx context.Context, clients types.Clients) error {
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
	if err := clients.K8sClient.Create(ctx, &htpasswd); err != nil {
		return err
	}

	return nil
}

func (r *Registry) createTLSSecret(ctx context.Context, clients types.Clients, registryIP string) error {
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
	if err := clients.K8sClient.Create(ctx, tlsSecret); err != nil {
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
