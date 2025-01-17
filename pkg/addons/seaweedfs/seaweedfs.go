package seaweedfs

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const releaseName = "seaweedfs"

var (
	//go:embed static/values.tpl.yaml
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
		panic(fmt.Errorf("failed to unmarshal metadata: %w", err))
	}
	Render()
}

func Render() {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(fmt.Sprintf("unable to unmarshal values: %v", err))
	}
	helmValues = hv
}

// SeaweedFS manages the installation of the SeaweedFS helm chart.
type SeaweedFS struct {
	namespace string
	isAirgap  bool
	isHA      bool
}

// Version returns the version of the SeaweedFS chart.
func (o *SeaweedFS) Version() (map[string]string, error) {
	return map[string]string{"SeaweedFS": "v" + Metadata.Version}, nil
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
func (o *SeaweedFS) GenerateHelmConfig(k0sCfg *k0sv1beta1.ClusterConfig, onlyDefaults bool) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	if !o.isAirgap || !o.isHA {
		return nil, nil, nil
	}

	chartConfig := ecv1beta1.Chart{
		Name:         releaseName,
		ChartName:    Metadata.Location,
		Version:      Metadata.Version,
		TargetNS:     o.namespace,
		ForceUpgrade: ptr.To(false),
		Order:        2,
	}

	if !onlyDefaults {
		var err error
		dataPath := filepath.Join(runtimeconfig.EmbeddedClusterSeaweedfsSubDir(), "ssd")
		helmValues, err = helm.SetValue(helmValues, "master.data.hostPathPrefix", dataPath)
		if err != nil {
			return nil, nil, fmt.Errorf("set helm values global.data.hostPathPrefix: %w", err)
		}
		logsPath := filepath.Join(runtimeconfig.EmbeddedClusterSeaweedfsSubDir(), "storage")
		helmValues, err = helm.SetValue(helmValues, "master.logs.hostPathPrefix", logsPath)
		if err != nil {
			return nil, nil, fmt.Errorf("set helm values global.logs.hostPathPrefix: %w", err)
		}
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []ecv1beta1.Chart{chartConfig}, nil, nil
}

func (a *SeaweedFS) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (o *SeaweedFS) GetAdditionalImages() []string {
	return nil
}

// Outro is executed after the cluster deployment.
func (o *SeaweedFS) Outro(ctx context.Context, cli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, releaseMetadata *types.ReleaseMetadata) error {
	// SeaweedFS is applied by the operator
	return nil
}

// New creates a new SeaweedFS addon.
func New(namespace string, isAirgap bool, isHA bool) (*SeaweedFS, error) {
	return &SeaweedFS{namespace: namespace, isAirgap: isAirgap, isHA: isHA}, nil
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
