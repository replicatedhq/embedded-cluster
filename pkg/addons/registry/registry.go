package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/certs"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName   = "docker-registry"
	tlsSecretName = "registry-tls"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL     = "https://url"
	ChartName    = "name"
	Version      = "v0.0.0"
	ImageVersion = "2.8.3"
)

var registryPassword = helpers.RandString(20)
var registryAddress = ""

var helmValues = map[string]interface{}{
	"replicaCount":     1,
	"fullnameOverride": "registry",
	"image": map[string]interface{}{
		"tag": ImageVersion,
	},
	"podAnnotations": map[string]interface{}{
		"backup.velero.io/backup-volumes": "data",
	},
	"storage": "filesystem",
	"persistence": map[string]interface{}{
		"enabled":      true,
		"size":         "10Gi",
		"accessMode":   "ReadWriteOnce",
		"storageClass": "openebs-hostpath",
	},
	"configData": map[string]interface{}{
		"auth": map[string]interface{}{
			"htpasswd": map[string]interface{}{
				"realm": "Registry",
				"path":  "/auth/htpasswd",
			},
		},
	},
	"extraVolumeMounts": []map[string]interface{}{
		{
			"name":      "auth",
			"mountPath": "/auth",
		},
	},
	"extraVolumes": []map[string]interface{}{
		{
			"name": "auth",
			"secret": map[string]interface{}{
				"secretName": "registry-auth",
			},
		},
	},
}

// Registry manages the installation of the Registry helm chart.
type Registry struct {
	namespace string
	config    v1beta1.ClusterConfig
	isAirgap  bool
}

// Version returns the version of the Registry chart.
func (o *Registry) Version() (map[string]string, error) {
	return map[string]string{"Registry": "v" + Version}, nil
}

func (a *Registry) Name() string {
	return "Registry"
}

// HostPreflights returns the host preflight objects found inside the Registry
// Helm Chart, this is empty as there is no host preflight on there.
func (o *Registry) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *Registry) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the Registry chart.
func (o *Registry) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !o.isAirgap {
		return nil, nil, nil
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  o.namespace,
		Order:     3,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "twuni",
		URL:  ChartURL,
	}

	// use a static cluster IP for the registry service based on the cluster CIDR range
	serviceCIDR := v1beta1.DefaultNetwork().ServiceCIDR
	if o.config.Spec != nil && o.config.Spec.Network != nil {
		serviceCIDR = o.config.Spec.Network.ServiceCIDR
	}
	registryServiceIP, err := helpers.GetLowerBandIP(serviceCIDR, 10)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster IP for registry service: %w", err)
	}
	helmValues["service"] = map[string]interface{}{
		"clusterIP": registryServiceIP.String(),
	}

	if !onlyDefaults {
		helmValues["tlsSecretName"] = tlsSecretName
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

func (o *Registry) GetAdditionalImages() []string {
	return nil
}

func (o *Registry) generateRegistryTLS(ctx context.Context, cli client.Client) (string, string, error) {
	nsn := types.NamespacedName{Name: "registry", Namespace: o.namespace}
	var svc corev1.Service
	if err := cli.Get(ctx, nsn, &svc); err != nil {
		return "", "", fmt.Errorf("unable to get registry service: %w", err)
	}

	opts := []certs.Option{
		certs.WithCommonName("registry"),
		certs.WithDuration(365 * 24 * time.Hour),
		certs.WithIPAddress(registryAddress),
	}

	for _, name := range []string{
		"registry",
		fmt.Sprintf("registry.%s.svc", o.namespace),
		fmt.Sprintf("registry.%s.svc.cluster.local", o.namespace),
	} {
		opts = append(opts, certs.WithDNSName(name))
	}

	builder, err := certs.NewBuilder(opts...)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cert builder: %w", err)
	}
	return builder.Generate()
}

// Outro is executed after the cluster deployment.
func (o *Registry) Outro(ctx context.Context, cli client.Client) error {
	if !o.isAirgap {
		return nil
	}

	loading := spinner.Start()
	loading.Infof("Waiting for Registry to be ready")

	if err := kubeutils.WaitForNamespace(ctx, cli, o.namespace); err != nil {
		loading.CloseWithError()
		return err
	}

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(registryPassword), bcrypt.DefaultCost)
	if err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to hash registry password: %w", err)
	}

	htpasswd := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-auth",
			Namespace: o.namespace,
			Labels: map[string]string{
				"app": "docker-registry", // this is the backup/restore label for the registry component
			},
		},
		StringData: map[string]string{
			"htpasswd": fmt.Sprintf("embedded-cluster:%s", string(hashPassword)),
		},
		Type: "Opaque",
	}
	err = cli.Create(ctx, &htpasswd)
	if err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to create registry-auth secret: %w", err)
	}

	if err := kubeutils.WaitForService(ctx, cli, o.namespace, "registry"); err != nil {
		loading.CloseWithError()
		return err
	}

	if err := InitRegistryClusterIP(ctx, cli, o.namespace); err != nil {
		loading.CloseWithError()
		return fmt.Errorf("failed to determine registry cluster IP: %w", err)
	}

	tlsCert, tlsKey, err := o.generateRegistryTLS(ctx, cli)
	if err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to generate registry tls: %w", err)
	}

	tlsSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: o.namespace,
			Labels: map[string]string{
				"app": "docker-registry", // this is the backup/restore label for the registry component
			},
		},
		StringData: map[string]string{"tls.crt": tlsCert, "tls.key": tlsKey},
		Type:       "Opaque",
	}
	if err := cli.Create(ctx, tlsSecret); err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to create %s secret: %w", tlsSecretName, err)
	}

	if err := kubeutils.WaitForDeployment(ctx, cli, o.namespace, "registry"); err != nil {
		loading.CloseWithError()
		return err
	}

	if err := airgap.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryAddress)); err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to add containerd registry config: %w", err)
	}

	loading.Closef("Registry is ready!")
	return nil
}

// New creates a new Registry addon.
func New(namespace string, config v1beta1.ClusterConfig, isAirgap bool) (*Registry, error) {
	return &Registry{namespace: namespace, config: config, isAirgap: isAirgap}, nil
}

func GetRegistryPassword() string {
	return registryPassword
}

func GetRegistryClusterIP() string {
	return registryAddress
}

func InitRegistryClusterIP(ctx context.Context, cli client.Client, namespace string) error {
	svc := corev1.Service{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "registry"}, &svc)
	if err != nil {
		return fmt.Errorf("failed to get registry service: %w", err)
	}

	registryAddress = svc.Spec.ClusterIP
	return nil
}
