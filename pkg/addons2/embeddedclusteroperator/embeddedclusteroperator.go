package embeddedclusteroperator

import (
	_ "embed"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"gopkg.in/yaml.v2"
)

type EmbeddedClusterOperator struct{}

const (
	releaseName = "embedded-cluster-operator"
	namespace   = "embedded-cluster"
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

func (a *EmbeddedClusterOperator) Name() string {
	return "Embedded Cluster Operator"
}

func (a *EmbeddedClusterOperator) ReleaseName() string {
	return releaseName
}

func (a *EmbeddedClusterOperator) Namespace() string {
	return namespace
}
