package adminconsole

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serializer runtime.Serializer
)

func init() {
	scheme := kubeutils.Scheme
	serializer = jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{
		Yaml: true,
	})
}

func (a *AdminConsole) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	// some resources are not part of the helm chart and need to be created before the chart is installed
	// TODO: move this to the helm chart
	if err := a.createPreRequisites(ctx, clients, inSpec, installOpts); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := a.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	helmOpts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    a.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    a.Namespace(),
		Labels:       getBackupLabels(),
	}

	if clients.IsDryRun {
		manifests, err := clients.HelmClient.Render(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "dry run render")
		}
		a.dryRunManifests = append(a.dryRunManifests, manifests...)
	} else {
		_, err = clients.HelmClient.Install(ctx, helmOpts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}

		// install the application
		if installOpts.KotsInstaller != nil {
			err := installOpts.KotsInstaller(writer)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *AdminConsole) createPreRequisites(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec, installOpts types.InstallOptions) error {
	if err := a.createNamespace(ctx, clients); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := a.createPasswordSecret(ctx, clients, installOpts); err != nil {
		return errors.Wrap(err, "create kots password secret")
	}

	if err := a.createTLSSecret(ctx, clients, installOpts); err != nil {
		return errors.Wrap(err, "create kots TLS secret")
	}

	if err := a.ensureCAConfigmap(ctx, clients, inSpec); err != nil {
		return errors.Wrap(err, "ensure CA configmap")
	}

	if inSpec.AirGap {
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

		registryIP, err := registry.GetRegistryClusterIP(serviceCIDR)
		if err != nil {
			return errors.Wrap(err, "get registry cluster IP")
		}
		if err := a.createRegistrySecret(ctx, clients, registryIP); err != nil {
			return errors.Wrap(err, "create registry secret")
		}
	}

	return nil
}

func (a *AdminConsole) createNamespace(ctx context.Context, clients types.Clients) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.Namespace(),
		},
	}

	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(&ns, b); err != nil {
			return errors.Wrap(err, "serialize namespace")
		}
		a.dryRunManifests = append(a.dryRunManifests, b.Bytes())
	} else {
		if err := clients.K8sClient.Create(ctx, &ns); client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return nil
}

func (a *AdminConsole) createPasswordSecret(ctx context.Context, clients types.Clients, installOpts types.InstallOptions) error {
	passwordBcrypt, err := bcrypt.GenerateFromPassword([]byte(installOpts.AdminConsolePassword), 10)
	if err != nil {
		return errors.Wrap(err, "generate bcrypt from password")
	}

	kotsPasswordSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-password",
			Namespace: a.Namespace(),
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		Data: map[string][]byte{
			"passwordBcrypt": []byte(passwordBcrypt),
		},
	}

	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(&kotsPasswordSecret, b); err != nil {
			return errors.Wrap(err, "serialize password secret")
		}
		a.dryRunManifests = append(a.dryRunManifests, b.Bytes())
	} else {
		err = clients.K8sClient.Create(ctx, &kotsPasswordSecret)
		if err != nil {
			return errors.Wrap(err, "create kotsadm-password secret")
		}
	}

	return nil
}

func (a *AdminConsole) createRegistrySecret(ctx context.Context, clients types.Clients, registryIP string) error {
	authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))
	authConfig := fmt.Sprintf(`{"auths":{"%s:5000":{"username": "embedded-cluster", "password": "%s", "auth": "%s"}}}`, registryIP, registry.GetRegistryPassword(), authString)

	registryCreds := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: a.Namespace(),
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		StringData: map[string]string{
			".dockerconfigjson": authConfig,
		},
		Type: "kubernetes.io/dockerconfigjson",
	}

	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(&registryCreds, b); err != nil {
			return errors.Wrap(err, "serialize registry secret")
		}
		a.dryRunManifests = append(a.dryRunManifests, b.Bytes())
	} else {
		err := clients.K8sClient.Create(ctx, &registryCreds)
		if err != nil {
			return errors.Wrap(err, "create registry-auth secret")
		}
	}

	return nil
}

func (a *AdminConsole) createTLSSecret(ctx context.Context, clients types.Clients, installOpts types.InstallOptions) error {
	if len(installOpts.TLSCertBytes) == 0 || len(installOpts.TLSKeyBytes) == 0 {
		return nil
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-tls",
			Namespace: a.Namespace(),
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
			Annotations: map[string]string{
				"acceptAnonymousUploads": "0",
			},
		},
		Type: "kubernetes.io/tls",
		Data: map[string][]byte{
			"tls.crt": installOpts.TLSCertBytes,
			"tls.key": installOpts.TLSKeyBytes,
		},
		StringData: map[string]string{
			"hostname": installOpts.Hostname,
		},
	}

	if clients.IsDryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(secret, b); err != nil {
			return errors.Wrap(err, "serialize TLS secret")
		}
		a.dryRunManifests = append(a.dryRunManifests, b.Bytes())
	} else {
		err := clients.K8sClient.Create(ctx, secret)
		if err != nil {
			return errors.Wrap(err, "create kotsadm-tls secret")
		}
	}

	return nil
}

func (a *AdminConsole) ensureCAConfigmap(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec) error {
	rc := runtimeconfig.New(inSpec.RuntimeConfig)

	if rc.HostCABundlePath() == "" {
		return nil
	}

	if clients.IsDryRun {
		checksum, err := calculateFileChecksum(rc.HostCABundlePath())
		if err != nil {
			return fmt.Errorf("calculate checksum: %w", err)
		}
		new, err := newCAConfigMap(rc.HostCABundlePath(), checksum)
		if err != nil {
			return fmt.Errorf("create map: %w", err)
		}
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(new, b); err != nil {
			return errors.Wrap(err, "serialize CA configmap")
		}
		a.dryRunManifests = append(a.dryRunManifests, b.Bytes())
		return nil
	}

	err := EnsureCAConfigmap(ctx, a.logf, clients.K8sClient, clients.MetadataClient, rc.HostCABundlePath())

	if k8serrors.IsRequestEntityTooLargeError(err) || errors.Is(err, fs.ErrNotExist) {
		// This can result in issues installing in environments with a MITM HTTP proxy.
		// NOTE: this cannot be a warning because it will mess up the spinner
		a.logf("WARNING: Failed to ensure kotsadm CA configmap: %v", err)
	} else if err != nil {
		return err
	}

	return nil
}
