package seaweedfs

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseName = "seaweedfs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var (
	helmValues map[string]interface{}
)

// SeaweedFS manages the installation of the SeaweedFS helm chart.
type SeaweedFS struct {
	namespace string
	config    v1beta1.ClusterConfig
	isAirgap  bool
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
func (o *SeaweedFS) GenerateHelmConfig(onlyDefaults bool) ([]eckinds.Chart, []eckinds.Repository, error) {
	if !o.isAirgap {
		return nil, nil, nil
	}

	chartConfig := eckinds.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  o.namespace,
		Order:     2,
	}

	repositoryConfig := eckinds.Repository{
		Name: "seaweedfs",
		URL:  ChartURL,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []eckinds.Chart{chartConfig}, []eckinds.Repository{repositoryConfig}, nil
}

func (o *SeaweedFS) GetAdditionalImages() []string {
	return nil
}

// Outro is executed after the cluster deployment.
func (o *SeaweedFS) Outro(ctx context.Context, cli client.Client) error {
	// SeaweedFS is applied by the operator
	return nil
}

// New creates a new SeaweedFS addon.
func New(namespace string, config v1beta1.ClusterConfig, isAirgap bool) (*SeaweedFS, error) {
	return &SeaweedFS{namespace: namespace, config: config, isAirgap: isAirgap}, nil
}

// WaitForReady waits for SeaweedFS to be ready.
func WaitForReady(ctx context.Context, cli client.Client, ns string, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		ready, err := kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-filer")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of seaweedfs-filer: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-master")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of seaweedfs-master: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-volume")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of seaweedfs-volume: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		if writer != nil {
			writer.Infof("Waiting for SeaweedFS to deploy: %d/3 ready", count)
		}
		return count == 3, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		return fmt.Errorf("error waiting for admin console: %v", lasterr)
	}
	return nil
}

func init() {
	helmValues = make(map[string]interface{})
	if err := yaml.Unmarshal(helmValuesYAML, &helmValues); err != nil {
		panic(fmt.Errorf("failed to unmarshal helm values: %w", err))
	}
}
