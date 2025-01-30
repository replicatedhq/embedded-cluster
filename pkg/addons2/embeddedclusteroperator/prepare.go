package embeddedclusteroperator

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

func (a *EmbeddedClusterOperator) prepare() error {
	if err := a.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (a *EmbeddedClusterOperator) generateHelmValues() error {
	helmValues["kotsVersion"] = adminconsole.Metadata.Version
	helmValues["embeddedClusterVersion"] = versions.Version
	helmValues["embeddedClusterK0sVersion"] = versions.K0sVersion

	return nil
}
