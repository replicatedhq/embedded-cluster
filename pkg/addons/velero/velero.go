package velero

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	chartURL              = "oci://proxy.replicated.com/anonymous/registry.replicated.com/ec-charts/velero"
	releaseName           = "velero"
	credentialsSecretName = "cloud-credentials"
)

// Overwritten by -ldflags in Makefile
var (
	VeleroChartVersion          = "v0.0.0"
	VeleroImageTag              = "v0.0.0"
	VeleroAWSPluginImageTag     = "v0.0.0"
	VeleroKubectlImageTag       = "v0.0.0"
	VeleroRestoreHelperImageTag = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"backupsEnabled":   false,
	"snapshotsEnabled": false,
	"deployNodeAgent":  true,
	"nodeAgent": map[string]interface{}{
		"podVolumePath": "/var/lib/k0s/kubelet/pods",
	},
	"image": map[string]interface{}{
		"repository": "proxy.replicated.com/anonymous/velero/velero",
		"tag":        VeleroImageTag,
	},
	"initContainers": []map[string]interface{}{
		{
			"name":            "velero-plugin-for-aws",
			"image":           fmt.Sprintf("proxy.replicated.com/anonymous/velero/velero-plugin-for-aws:%s", VeleroAWSPluginImageTag),
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
	"kubectl": map[string]interface{}{
		"image": map[string]interface{}{
			"repository": "proxy.replicated.com/anonymous/bitnami/kubectl",
			"tag":        VeleroKubectlImageTag,
		},
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
	return map[string]string{"Velero": "v" + VeleroChartVersion}, nil
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
func (o *Velero) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !o.isEnabled {
		return nil, nil, nil
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: chartURL,
		Version:   VeleroChartVersion,
		TargetNS:  o.namespace,
		Order:     3,
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

	return []v1beta1.Chart{chartConfig}, nil, nil
}

func (o *Velero) GetAdditionalImages() []string {
	return []string{
		fmt.Sprintf(
			"proxy.replicated.com/anonymous/bitnami/kubectl:%s",
			VeleroKubectlImageTag,
		),
		fmt.Sprintf(
			"proxy.replicated.com/anonymous/velero/velero-restore-helper:%s",
			VeleroRestoreHelperImageTag,
		),
	}
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
