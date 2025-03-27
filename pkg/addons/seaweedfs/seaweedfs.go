package seaweedfs

import (
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v3"
)

type SeaweedFS struct {
	ServiceCIDR         string
	ProxyRegistryDomain string
}

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
		panic(errors.Wrap(err, "unable to unmarshal metadata"))
	}
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal values"))
	}
	helmValues = hv
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

func (s *SeaweedFS) ChartLocation() string {
	if s.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", s.ProxyRegistryDomain, 1)
}
