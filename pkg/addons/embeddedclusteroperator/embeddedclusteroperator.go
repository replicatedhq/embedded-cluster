package embeddedclusteroperator

import (
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	releaseName = "embedded-cluster-operator"
	namespace   = "embedded-cluster"
)

var (
	serializer runtime.Serializer
)

func init() {
	scheme := kubeutils.Scheme
	serializer = jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{
		Yaml: true,
	})
}

var _ types.AddOn = (*EmbeddedClusterOperator)(nil)

type EmbeddedClusterOperator struct {
	IsAirgap              bool
	Proxy                 *ecv1beta1.ProxySpec
	HostCABundlePath      string
	ChartLocationOverride string
	ChartVersionOverride  string
	ImageRepoOverride     string
	ImageTagOverride      string
	UtilsImageOverride    string
	ProxyRegistryDomain   string

	// DryRun is a flag to enable dry-run mode for Velero.
	// If true, Velero will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

func (e *EmbeddedClusterOperator) Name() string {
	return "Embedded Cluster Operator"
}

func (e *EmbeddedClusterOperator) Version() string {
	return e.ChartVersion()
}

func (e *EmbeddedClusterOperator) ReleaseName() string {
	return releaseName
}

func (e *EmbeddedClusterOperator) Namespace() string {
	return namespace
}

func (e *EmbeddedClusterOperator) ChartLocation() string {
	location := Metadata.Location
	if e.ChartLocationOverride != "" {
		location = e.ChartLocationOverride
	}
	if e.ProxyRegistryDomain == "" {
		return location
	}
	return strings.Replace(location, "proxy.replicated.com", e.ProxyRegistryDomain, 1)
}

func (e *EmbeddedClusterOperator) ChartVersion() string {
	if e.ChartVersionOverride != "" {
		return e.ChartVersionOverride
	}
	return Metadata.Version
}

func (v *EmbeddedClusterOperator) DryRunManifests() [][]byte {
	return v.dryRunManifests
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "embedded-cluster-operator",
	}
}
