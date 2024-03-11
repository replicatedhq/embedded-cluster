package seaweedfs

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "seaweedfs"
	namespace   = "seaweedfs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"master": map[string]interface{}{
		"replicas": 1,
		"data": map[string]interface{}{
			"type":         "persistentVolumeClaim",
			"size":         "1Gi",
			"storageClass": "local",
		},
		"disableHttp": true,
	},
	"volume": map[string]interface{}{
		"replicas": 1,
		"dataDirs": []map[string]interface{}{
			{
				"name":         "data1",
				"type":         "persistentVolumeClaim",
				"size":         "10Gi",
				"storageClass": "local",
				"maxVolumes":   0,
			},
		},
	},
	"filer": map[string]interface{}{
		"data": map[string]interface{}{
			"type":         "persistentVolumeClaim",
			"size":         "1Gi",
			"storageClass": "local",
		},
		"s3": map[string]interface{}{
			"enabled":              true,
			"enableAuth":           true,
			"existingConfigSecret": "seaweedfs-s3-access-secret",
		},
		"disableHttp": true,
	},
}

// SeaweedFS manages the installation of the SeaweedFS helm chart.
type SeaweedFS struct {
	isAirgap bool
}

// Version returns the version of the SeaweedFS chart.
func (o *SeaweedFS) Version() (map[string]string, error) {
	return map[string]string{"SeaweedFS": "v" + Version}, nil
}

func (a *SeaweedFS) Name() string {
	return "SeaweedFS"
}

// HostPreflights returns the host preflight objects found inside the SeaweedFS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *SeaweedFS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *SeaweedFS) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the SeaweedFS chart.
func (o *SeaweedFS) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	//if !o.isAirgap {
	//	return nil, nil, nil
	//}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  namespace,
		Order:     2,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "seaweedfs",
		URL:  ChartURL,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

// Outro is executed after the cluster deployment.
func (o *SeaweedFS) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	loading.Infof("Waiting for SeaweedFS to be ready")

	accessSecretTemplate := `'{"identities":[{"name":"anvAdmin","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Admin","Read","Write"]},{"name":"anvReadOnly","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Read"]}]}'`
	// generate 4 random strings for access keys and secret keys
	accessKey1, secretKey1, accessKey2, secretKey2 := helpers.RandString(20), helpers.RandString(40), helpers.RandString(20), helpers.RandString(40)
	accessSecretString := fmt.Sprintf(accessSecretTemplate, accessKey1, secretKey1, accessKey2, secretKey2)
	accessSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seaweedfs-s3-access-secret",
			Namespace: namespace,
		},
		StringData: map[string]string{
			"seaweedfs_s3_config": accessSecretString,
		},
		Type: "Opaque",
	}
	err := cli.Create(ctx, &accessSecret)
	if err != nil {
		loading.Close()
		return fmt.Errorf("unable to create seaweedfs-s3-access-secret: %w", err)
	}

	if err = kubeutils.WaitForDeployment(ctx, cli, namespace, "seaweedfs"); err != nil {
		loading.Close()
		return err
	}
	loading.Closef("SeaweedFS is ready!")
	return nil
}

// New creates a new SeaweedFS addon.
func New(isAirgap bool) (*SeaweedFS, error) {
	return &SeaweedFS{isAirgap: isAirgap}, nil
}
