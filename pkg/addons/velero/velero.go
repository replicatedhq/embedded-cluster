package velero

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName           = "velero"
	credentialsSecretName = "cloud-credentials"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL     = "https://url"
	ChartName    = "name"
	Version      = "v0.0.0"
	VeleroTag    = "v0.0.0"
	AwsPluginTag = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"backupsEnabled":   false,
	"snapshotsEnabled": false,
	"deployNodeAgent":  true,
	"nodeAgent": map[string]interface{}{
		"podVolumePath": "/var/lib/k0s/kubelet/pods",
	},
	"image": map[string]interface{}{
		"tag": VeleroTag,
	},
	"initContainers": []map[string]interface{}{
		{
			"name":            "velero-plugin-for-aws",
			"image":           fmt.Sprintf("velero/velero-plugin-for-aws:%s", AwsPluginTag),
			"imagePullPolicy": "IfNotPresent",
			"volumeMounts": []map[string]interface{}{
				{
					"mountPath": "/target",
					"name":      "plugins",
				},
			},
		},
	},
	"credentials": map[string]interface{}{
		"existingSecret": credentialsSecretName,
	},
}

// Velero manages the installation of the Velero helm chart.
type Velero struct {
	namespace string
	isEnabled bool
	proxyEnv  map[string]string
}

// Version returns the version of the Velero chart.
func (o *Velero) Version() (map[string]string, error) {
	return map[string]string{"Velero": "v" + Version}, nil
}

func (a *Velero) Name() string {
	return "Velero"
}

// HostPreflights returns the host preflight objects found inside the Velero
// Helm Chart, this is empty as there is no host preflight on there.
func (o *Velero) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *Velero) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the Velero chart.
func (o *Velero) GenerateHelmConfig(onlyDefaults bool) ([]eckinds.Chart, []v1beta1.Repository, error) {
	if !o.isEnabled {
		return nil, nil, nil
	}

	chartConfig := eckinds.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  o.namespace,
		Order:     3,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "vmware-tanzu",
		URL:  ChartURL,
	}

	kubectlVersion := semver.MustParse(defaults.K0sVersion)
	helmValues["kubectl"] = map[string]interface{}{
		"image": map[string]interface{}{
			"tag": fmt.Sprintf("%d.%d", kubectlVersion.Major(), kubectlVersion.Minor()),
		},
	}

	if len(o.proxyEnv) > 0 {
		extraEnvVars := map[string]interface{}{}
		for k, v := range o.proxyEnv {
			extraEnvVars[k] = v
		}
		helmValues["configuration"] = map[string]interface{}{
			"extraEnvVars": extraEnvVars,
		}
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []eckinds.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

func (o *Velero) GetAdditionalImages() []string {
	return []string{fmt.Sprintf("velero/velero-restore-helper:%s", VeleroTag)}
}

// Outro is executed after the cluster deployment.
func (o *Velero) Outro(ctx context.Context, cli client.Client) error {
	if !o.isEnabled {
		return nil
	}

	loading := spinner.Start()
	loading.Infof("Waiting for Velero to be ready")

	if err := kubeutils.WaitForNamespace(ctx, cli, o.namespace); err != nil {
		loading.Close()
		return err
	}

	credentialsSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialsSecretName,
			Namespace: o.namespace,
		},
		Type: "Opaque",
	}
	if err := cli.Create(ctx, &credentialsSecret); err != nil {
		loading.Close()
		return fmt.Errorf("unable to create %s secret: %w", credentialsSecretName, err)
	}

	if err := kubeutils.WaitForDeployment(ctx, cli, o.namespace, "velero"); err != nil {
		loading.Close()
		return fmt.Errorf("timed out waiting for Velero to deploy: %v", err)
	}

	if err := kubeutils.WaitForDaemonset(ctx, cli, o.namespace, "node-agent"); err != nil {
		loading.Close()
		return fmt.Errorf("timed out waiting for node-agent to deploy: %v", err)
	}

	loading.Closef("Velero is ready!")
	return nil
}

// New creates a new Velero addon.
func New(namespace string, isEnabled bool, proxyEnv map[string]string) (*Velero, error) {
	return &Velero{
		namespace: namespace,
		isEnabled: isEnabled,
		proxyEnv:  proxyEnv,
	}, nil
}
