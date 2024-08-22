package velero

import (
	"context"
	_ "embed"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName           = "velero"
	credentialsSecretName = "cloud-credentials"
)

var (
	//go:embed static/values.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}
	helmValues = make(map[string]interface{})
	if err := yaml.Unmarshal(rawvalues, &helmValues); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}
}

// Velero manages the installation of the Velero helm chart.
type Velero struct {
	namespace string
	isEnabled bool
	proxyEnv  map[string]string
}

// Version returns the version of the Velero chart.
func (o *Velero) Version() (map[string]string, error) {
	return map[string]string{"Velero": "v" + Metadata.Version}, nil
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
func (o *Velero) GenerateHelmConfig(k0sCfg *k0sv1beta1.ClusterConfig, onlyDefaults bool) ([]ecv1beta1.Chart, []ecv1beta1.Repository, error) {
	if !o.isEnabled {
		return nil, nil, nil
	}

	chartConfig := ecv1beta1.Chart{
		Name:      releaseName,
		ChartName: Metadata.Location,
		Version:   Metadata.Version,
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

	return []ecv1beta1.Chart{chartConfig}, nil, nil
}

func (a *Velero) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (o *Velero) GetAdditionalImages() []string {
	var images []string
	if image, ok := Metadata.Images["velero-restore-helper"]; ok {
		images = append(images, image.String())
	}
	if image, ok := Metadata.Images["kubectl"]; ok {
		images = append(images, image.String())
	}
	return images
}

// Outro is executed after the cluster deployment.
func (o *Velero) Outro(ctx context.Context, cli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, releaseMetadata *types.ReleaseMetadata) error {
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
