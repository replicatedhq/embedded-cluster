package velero

import (
	_ "embed"
	"log/slog"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
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
	logf types.LogFunc

	dryRunManifests [][]byte
}

type Option func(*Velero)

func New(opts ...Option) *Velero {
	addon := &Velero{}
	for _, opt := range opts {
		opt(addon)
	}
	if addon.logf == nil {
		addon.logf = slog.Info
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *Velero) {
		a.logf = logf
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
