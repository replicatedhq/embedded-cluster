package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
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
	releaseName             = "docker-registry"
	tlsSecretName           = "registry-tls"
	seaweedfsS3RWSecretName = "seaweedfs-s3-rw"

	registryLowerBandIPIndex = 10
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

var helmValuesHA = map[string]interface{}{
	"replicaCount":     2,
	"fullnameOverride": "registry",
	"image": map[string]interface{}{
		"tag": ImageVersion,
	},
	"storage": "s3",
	"s3": map[string]interface{}{
		"region": "us-east-1",
		// regionEndpoint is set by the operator to a hardcoded lower band IP
		"regionEndpoint": "DYNAMIC",
		"bucket":         "registry",
		"rootdirectory":  "/registry",
		"encrypt":        false,
		"secure":         false, // This does not work
	},
	"secrets": map[string]interface{}{
		"s3": map[string]interface{}{
			"secretRef": seaweedfsS3RWSecretName,
		},
	},
	"configData": map[string]interface{}{
		"auth": map[string]interface{}{
			"htpasswd": map[string]interface{}{
				"realm": "Registry",
				"path":  "/auth/htpasswd",
			},
		},
		"storage": map[string]interface{}{
			"s3": map[string]interface{}{
				// There is a bug in the helm chart that REGISTRY_STORAGE_S3_SECURE is not set if
				// false yet it defaults to true
				"secure": false,
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
	"affinity": map[string]interface{}{
		"podAntiAffinity": map[string]interface{}{
			"requiredDuringSchedulingIgnoredDuringExecution": []map[string]interface{}{
				{
					"labelSelector": map[string]interface{}{
						"matchExpressions": []map[string]interface{}{
							{
								"key":      "app",
								"operator": "In",
								"values":   []string{"docker-registry"},
							},
						},
					},
					"topologyKey": "kubernetes.io/hostname",
				},
			},
		},
	},
}

// Registry manages the installation of the Registry helm chart.
type Registry struct {
	namespace string
	config    v1beta1.ClusterConfig
	isAirgap  bool
	isHA      bool
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
func (o *Registry) GenerateHelmConfig(onlyDefaults bool) ([]eckinds.Chart, []eckinds.Repository, error) {
	if !o.isAirgap {
		return nil, nil, nil
	}

	chartConfig := eckinds.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  o.namespace,
		Order:     3,
	}

	repositoryConfig := eckinds.Repository{
		Name: "twuni",
		URL:  ChartURL,
	}

	var values map[string]interface{}
	if o.isHA {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	// use a static cluster IP for the registry service based on the cluster CIDR range
	serviceCIDR := v1beta1.DefaultNetwork().ServiceCIDR
	if o.config.Spec != nil && o.config.Spec.Network != nil {
		serviceCIDR = o.config.Spec.Network.ServiceCIDR
	}
	registryServiceIP, err := helpers.GetLowerBandIP(serviceCIDR, registryLowerBandIPIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster IP for registry service: %w", err)
	}
	values["service"] = map[string]interface{}{
		"clusterIP": registryServiceIP.String(),
	}

	if !onlyDefaults {
		values["tlsSecretName"] = tlsSecretName
	}

	valuesStringData, err := yaml.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []eckinds.Chart{chartConfig}, []eckinds.Repository{repositoryConfig}, nil
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

func (o *Registry) generateRegistryMigrationRole(ctx context.Context, cli client.Client) error {
	newRole := rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-role",
			Namespace: o.namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create"},
			},
		},
	}
	err := cli.Create(ctx, &newRole)
	if err != nil {
		return fmt.Errorf("unable to create registry-data-migration-role: %w", err)
	}

	newServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-serviceaccount",
			Namespace: o.namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
	}
	err = cli.Create(ctx, &newServiceAccount)
	if err != nil {
		return fmt.Errorf("unable to create registry-data-migration-serviceaccount: %w", err)
	}

	newRoleBinding := rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-rolebinding",
			Namespace: o.namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     "registry-data-migration-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "registry-data-migration-serviceaccount",
				Namespace: o.namespace,
			},
		},
	}

	err = cli.Create(ctx, &newRoleBinding)
	if err != nil {
		return fmt.Errorf("unable to create registry-data-migration-rolebinding: %w", err)
	}

	return nil
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

	if err := o.generateRegistryMigrationRole(ctx, cli); err != nil {
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
func New(namespace string, config v1beta1.ClusterConfig, isAirgap bool, isHA bool) (*Registry, error) {
	return &Registry{namespace: namespace, config: config, isAirgap: isAirgap, isHA: isHA}, nil
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
