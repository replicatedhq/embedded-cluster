package registry

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "docker-registry"
	namespace   = "registry"
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
	"service": map[string]interface{}{
		"clusterIP": "10.103.113.120", // TODO: make dynamic based on k0s cluster CIDR range (lower end)
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
	isAirgap bool
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
		TargetNS:  namespace,
		Order:     3,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "twuni",
		URL:  ChartURL,
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

// Outro is executed after the cluster deployment.
func (o *Registry) Outro(ctx context.Context, cli client.Client) error {
	if !o.isAirgap {
		return nil
	}

	loading := spinner.Start()
	loading.Infof("Waiting for Registry to be ready")

	if err := kubeutils.WaitForNamespace(ctx, cli, namespace); err != nil {
		loading.Close()
		return err
	}

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(registryPassword), bcrypt.DefaultCost)
	if err != nil {
		loading.Close()
		return fmt.Errorf("unable to hash registry password: %w", err)
	}

	htpasswd := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-auth",
			Namespace: namespace,
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
		loading.Close()
		return fmt.Errorf("unable to create registry-auth secret: %w", err)
	}

	if err := kubeutils.WaitForDeployment(ctx, cli, namespace, "registry"); err != nil {
		loading.Close()
		return err
	}
	if err := kubeutils.WaitForService(ctx, cli, namespace, "registry"); err != nil {
		loading.Close()
		return err
	}

	if err := initRegistryClusterIP(ctx, cli); err != nil {
		loading.Close()
		return fmt.Errorf("failed to determine registry cluster IP: %w", err)
	}

	if err := airgap.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryAddress)); err != nil {
		loading.Close()
		return fmt.Errorf("failed to add insecure registry: %w", err)
	}

	loading.Closef("Registry is ready!")
	return nil
}

// New creates a new Registry addon.
func New(isAirgap bool) (*Registry, error) {
	return &Registry{isAirgap: isAirgap}, nil
}

func GetRegistryPassword() string {
	return registryPassword
}

func GetRegistryClusterIP() string {
	return registryAddress
}

func initRegistryClusterIP(ctx context.Context, cli client.Client) error {
	svc := corev1.Service{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "registry"}, &svc)
	if err != nil {
		return fmt.Errorf("failed to get registry service: %w", err)
	}

	registryAddress = svc.Spec.ClusterIP
	return nil
}
