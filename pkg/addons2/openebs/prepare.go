package openebs

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (a *OpenEBS) prepare() error {
	if err := a.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (a *OpenEBS) generateHelmValues() error {
	var err error
	helmValues, err = helm.SetValue(helmValues, `["localpv-provisioner"].localpv.basePath`, runtimeconfig.EmbeddedClusterOpenEBSLocalSubDir())
	if err != nil {
		return errors.Wrap(err, "set localpv-provisioner.localpv.basePath")
	}

	return nil
}
