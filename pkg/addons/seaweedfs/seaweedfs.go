package seaweedfs

import (
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	_releaseName = "seaweedfs"
	_namespace   = constants.SeaweedFSNamespace

	// _s3SVCName is the name of the Seaweedfs S3 service managed by the operator.
	// HACK: This service has a hardcoded service IP shared by the cli and operator as it is used
	// by the registry to redirect requests for blobs.
	_s3SVCName = "ec-seaweedfs-s3"

	// _lowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	_lowerBandIPIndex = 11

	// _s3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the values-ha.yaml file in the release metadata.
	_s3SecretName = "secret-seaweedfs-s3"
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

var _ types.AddOn = (*SeaweedFS)(nil)

type SeaweedFS struct {
	ServiceCIDR      string
	SeaweedFSDataDir string

	// DryRun is a flag to enable dry-run mode for SeaweedFS.
	// If true, SeaweedFS will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

func (s *SeaweedFS) Name() string {
	return "SeaweedFS"
}

func (s *SeaweedFS) Version() string {
	return Metadata.Version
}

func (s *SeaweedFS) ReleaseName() string {
	return _releaseName
}

func (s *SeaweedFS) Namespace() string {
	return _namespace
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

func (s *SeaweedFS) DryRunManifests() [][]byte {
	return s.dryRunManifests
}
