package velero

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
	releaseName           = "velero"
	namespace             = runtimeconfig.VeleroNamespace
	credentialsSecretName = "cloud-credentials"
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

var _ types.AddOn = (*Velero)(nil)

type Velero struct {
	logf          types.LogFunc
	kcli          client.Client
	mcli          metadata.Interface
	hcli          helm.Client
	runtimeConfig runtimeconfig.RuntimeConfig

	dryRunManifests [][]byte
}

type Option func(*Velero)

func New(opts ...Option) *Velero {
	addon := &Velero{}
	for _, opt := range opts {
		opt(addon)
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *Velero) {
		a.logf = logf
	}
}

func WithClients(kcli client.Client, mcli metadata.Interface, hcli helm.Client) Option {
	return func(a *Velero) {
		a.kcli = kcli
		a.mcli = mcli
		a.hcli = hcli
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) Option {
	return func(a *Velero) {
		a.runtimeConfig = rc
	}
}

func (v *Velero) Name() string {
	return "Velero"
}

func (v *Velero) Version() string {
	return Metadata.Version
}

func (v *Velero) ReleaseName() string {
	return releaseName
}

func (v *Velero) Namespace() string {
	return namespace
}

func (v *Velero) ChartLocation(domains ecv1beta1.Domains) string {
	if domains.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
}

func (v *Velero) DryRunManifests() [][]byte {
	return v.dryRunManifests
}
