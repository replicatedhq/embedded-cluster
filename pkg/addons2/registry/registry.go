package registry

import (
	_ "embed"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v3"
)

type Registry struct {
	ServiceCIDR string
}

const (
	releaseName              = "docker-registry"
	namespace                = runtimeconfig.RegistryNamespace
	tlsSecretName            = "registry-tls"
	registryLowerBandIPIndex = 10
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

	registryPassword = helpers.RandString(20)
	registryAddress  = ""
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

func (r *Registry) Name() string {
	return "Registry"
}

func GetRegistryPassword() string {
	return registryPassword
}

func GetRegistryClusterIP() string {
	return registryAddress
}
