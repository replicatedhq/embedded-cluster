package velero

import (
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v3"
)

type Velero struct {
	Proxy               *ecv1beta1.ProxySpec
	ProxyRegistryDomain string
}

const (
	releaseName           = "velero"
	namespace             = runtimeconfig.VeleroNamespace
	credentialsSecretName = "cloud-credentials"
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

func (v *Velero) ChartLocation() string {
	if v.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", v.ProxyRegistryDomain, 1)
}
