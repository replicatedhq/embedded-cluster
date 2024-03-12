package seaweedfs

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
			"type":           "hostPath",
			"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/master",
		},
		"logs": map[string]interface{}{
			"type": "none",
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
				"storageClass": "openebs-hostpath",
				"maxVolumes":   0,
			},
		},
	},
	"filer": map[string]interface{}{
		"data": map[string]interface{}{
			"type": "persistentVolumeClaim",
			"size": "1Gi",
		},
		"logs": map[string]interface{}{
			"type": "none",
		},
		"s3": map[string]interface{}{
			"enabled":              true,
			"enableAuth":           true,
			"existingConfigSecret": "seaweedfs-s3-access-secret",
		},
	},
}

var (
	rwAccessKey = helpers.RandString(20)
	rwSecretKey = helpers.RandString(40)
	roAccessKey = helpers.RandString(20)
	roSecretKey = helpers.RandString(40)
)

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
	//if !o.isAirgap {
	//	return nil
	//}

	loading := spinner.Start()
	loading.Infof("Waiting for SeaweedFS to be ready")
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace); err != nil {
		loading.Close()
		return err
	}
	loading.Infof("SeaweedFS namespace is ready")

	accessSecretTemplate := `{"identities":[{"name":"anvAdmin","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Admin","Read","Write"]},{"name":"anvReadOnly","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Read"]}]}`
	accessSecretString := fmt.Sprintf(accessSecretTemplate, rwAccessKey, rwSecretKey, roAccessKey, roSecretKey)
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

	var lasterr error
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	if err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		for _, name := range []string{"seaweedfs-master", "seaweedfs-volume", "seaweedfs-filer"} {
			ready, err := kubeutils.IsStatefulSetReady(ctx, cli, namespace, name)
			if err != nil {
				lasterr = fmt.Errorf("error checking status of %s: %v", name, err)
				return false, nil
			}
			if ready {
				count++
			}
		}
		loading.Infof("Waiting for SeaweedFS to deploy: %d/3 ready", count)
		return count == 3, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		loading.Close()
		return fmt.Errorf("error waiting for SeaweedFS: %v", lasterr)
	}

	loading.Closef("SeaweedFS is ready!")
	return nil
}

// New creates a new SeaweedFS addon.
func New(isAirgap bool) (*SeaweedFS, error) {
	return &SeaweedFS{isAirgap: isAirgap}, nil
}

func GetRWInfo() (string, string) {
	return rwAccessKey, rwSecretKey
}
