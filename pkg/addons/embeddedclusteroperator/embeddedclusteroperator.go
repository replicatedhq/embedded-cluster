package embeddedclusteroperator

import (
	_ "embed"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	logf          types.LogFunc
	kcli          client.Client
	mcli          metadata.Interface
	hcli          helm.Client
	runtimeConfig runtimeconfig.RuntimeConfig

	dryRunManifests [][]byte

	ChartLocationOverride string
	ChartVersionOverride  string
	ImageRepoOverride     string
	ImageTagOverride      string
	UtilsImageOverride    string
}

type Option func(*EmbeddedClusterOperator)

func New(opts ...Option) *EmbeddedClusterOperator {
	addon := &EmbeddedClusterOperator{}
	for _, opt := range opts {
		opt(addon)
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *EmbeddedClusterOperator) {
		a.logf = logf
	}
}

func WithClients(kcli client.Client, mcli metadata.Interface, hcli helm.Client) Option {
	return func(a *EmbeddedClusterOperator) {
		a.kcli = kcli
		a.mcli = mcli
		a.hcli = hcli
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) Option {
	return func(a *EmbeddedClusterOperator) {
		a.runtimeConfig = rc
	}
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
