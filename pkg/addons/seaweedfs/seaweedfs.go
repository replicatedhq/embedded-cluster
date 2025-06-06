package seaweedfs

import (
	_ "embed"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseName = "seaweedfs"
	namespace   = runtimeconfig.SeaweedFSNamespace

	// s3SVCName is the name of the Seaweedfs S3 service managed by the operator.
	// HACK: This service has a hardcoded service IP shared by the cli and operator as it is used
	// by the registry to redirect requests for blobs.
	s3SVCName = "ec-seaweedfs-s3"

	// lowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	lowerBandIPIndex = 11

	// s3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the values-ha.yaml file in the release metadata.
	s3SecretName = "secret-seaweedfs-s3"
)

var _ types.AddOn = (*SeaweedFS)(nil)

type SeaweedFS struct {
	logf          types.LogFunc
	kcli          client.Client
	mcli          metadata.Interface
	hcli          helm.Client
	runtimeConfig runtimeconfig.RuntimeConfig

	dryRunManifests [][]byte
}

type Option func(*SeaweedFS)

func New(opts ...Option) *SeaweedFS {
	addon := &SeaweedFS{}
	for _, opt := range opts {
		opt(addon)
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *SeaweedFS) {
		a.logf = logf
	}
}

func WithClients(kcli client.Client, mcli metadata.Interface, hcli helm.Client) Option {
	return func(a *SeaweedFS) {
		a.kcli = kcli
		a.mcli = mcli
		a.hcli = hcli
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) Option {
	return func(a *SeaweedFS) {
		a.runtimeConfig = rc
	}
}

func (s *SeaweedFS) Name() string {
	return "SeaweedFS"
}

func (s *SeaweedFS) Version() string {
	return Metadata.Version
}

func (s *SeaweedFS) ReleaseName() string {
	return releaseName
}

func (s *SeaweedFS) Namespace() string {
	return namespace
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": "seaweedfs",
	}
}

func (s *SeaweedFS) ChartLocation(domains ecv1beta1.Domains) string {
	if domains.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
}
