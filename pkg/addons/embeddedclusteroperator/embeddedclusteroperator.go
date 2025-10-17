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
	_releaseName = "embedded-cluster-operator"
	_namespace   = "embedded-cluster"
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
	ClusterID        string
	IsAirgap         bool
	Proxy            *ecv1beta1.ProxySpec
	HostCABundlePath string
	KotsadmNamespace string

	ChartLocationOverride string
	ChartVersionOverride  string
	ImageRepoOverride     string
	ImageTagOverride      string
	UtilsImageOverride    string

	// DryRun is a flag to enable dry-run mode for Velero.
	// If true, Velero will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

func (e *EmbeddedClusterOperator) Name() string {
	return "Runtime Operator"
}

func (e *EmbeddedClusterOperator) Version() string {
	return e.ChartVersion()
}

func (e *EmbeddedClusterOperator) ReleaseName() string {
	return _releaseName
}

func (e *EmbeddedClusterOperator) Namespace() string {
	return _namespace
}

func (e *EmbeddedClusterOperator) ChartLocation(domains ecv1beta1.Domains) string {
	location := Metadata.Location
	if e.ChartLocationOverride != "" {
		location = e.ChartLocationOverride
	}
	if domains.ProxyRegistryDomain == "" {
		return location
	}
	return strings.Replace(location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
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
