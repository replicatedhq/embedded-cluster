package velero

import (
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	_releaseName = "velero"
	_namespace   = runtimeconfig.VeleroNamespace

	_credentialsSecretName = "cloud-credentials"
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
	Proxy *ecv1beta1.ProxySpec

	// DryRun is a flag to enable dry-run mode for Velero.
	// If true, Velero will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

func (v *Velero) Name() string {
	return "disaster recovery"
}

func (v *Velero) Version() string {
	return Metadata.Version
}

func (v *Velero) ReleaseName() string {
	return _releaseName
}

func (v *Velero) Namespace() string {
	return _namespace
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
